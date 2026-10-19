//go:build rb2go_prelude

package prelude

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// File::FNM_* bits, MRI's values.
const (
	rbFnmNoescape = 1
	rbFnmPathname = 2
	rbFnmDotmatch = 4
	rbFnmCasefold = 8
	rbFnmExtglob  = 16
)

// rbFnmatch is File.fnmatch, following MRI's fnmatch_helper so Homebrew's link globs match as under MRI.
func rbFnmatch(pattern, path string, flags int) bool {
	if flags&rbFnmExtglob != 0 {
		for _, p := range rbBraceExpand(pattern, flags&rbFnmNoescape == 0) {
			if rbFnmatch(p, path, flags&^rbFnmExtglob) {
				return true
			}
		}
		return false
	}
	if flags&rbFnmPathname == 0 {
		return rbFnmSeg(pattern, path, flags, true)
	}
	return rbFnmPath(strings.Split(pattern, "/"), strings.Split(path, "/"), flags)
}

// rbFnmPath matches segment lists, `**` segments spanning zero or more path segments.
func rbFnmPath(ps, ss []string, flags int) bool {
	for len(ps) > 0 {
		if ps[0] == "**" && len(ps) > 1 { // `**/`: any directories, none starting with `.` unless FNM_DOTMATCH
			for i := 0; i <= len(ss); i++ {
				if rbFnmPath(ps[1:], ss[i:], flags) {
					return true
				}
				if i < len(ss) && strings.HasPrefix(ss[i], ".") && flags&rbFnmDotmatch == 0 {
					return false
				}
			}
			return false
		}
		if len(ss) == 0 || !rbFnmSeg(ps[0], ss[0], flags, true) {
			return false
		}
		ps, ss = ps[1:], ss[1:]
	}
	return len(ss) == 0
}

// rbFnmSeg matches one pattern against one string; start is whether s begins a path (or segment), for the period rule.
func rbFnmSeg(p, s string, flags int, start bool) bool {
	period := start && flags&rbFnmDotmatch == 0 && strings.HasPrefix(s, ".")
	escape := flags&rbFnmNoescape == 0
	fold := flags&rbFnmCasefold != 0
	slash := flags&rbFnmPathname != 0
	for len(p) > 0 {
		switch p[0] {
		case '*':
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if period {
				return false
			}
			for i := 0; i <= len(s); i++ {
				if rbFnmSeg(p, s[i:], flags, false) {
					return true
				}
				if i < len(s) && slash && s[i] == '/' {
					return false
				}
			}
			return false
		case '?':
			if len(s) == 0 || period || slash && s[0] == '/' {
				return false
			}
			_, n := utf8.DecodeRuneInString(s)
			p, s = p[1:], s[n:]
		case '[':
			if len(s) == 0 || period || slash && s[0] == '/' {
				return false
			}
			r, n := utf8.DecodeRuneInString(s)
			ok, rest, valid := rbFnmClass(p[1:], r, escape, fold)
			if !valid || !ok { // an unclosed `[` matches nothing, as MRI's
				return false
			}
			p, s = rest, s[n:]
		default:
			if p[0] == '\\' && escape && len(p) > 1 {
				p = p[1:]
			}
			pr, pn := utf8.DecodeRuneInString(p)
			if len(s) == 0 {
				return false
			}
			sr, sn := utf8.DecodeRuneInString(s)
			if pr != sr && (!fold || unicode.ToLower(pr) != unicode.ToLower(sr)) {
				return false
			}
			p, s = p[pn:], s[sn:]
		}
		period = false
	}
	return len(s) == 0
}

// rbFnmClass reports whether r matches the class, the pattern after its `]`, and whether it was closed.
func rbFnmClass(p string, r rune, escape, fold bool) (bool, string, bool) {
	neg := len(p) > 0 && (p[0] == '!' || p[0] == '^')
	if neg {
		p = p[1:]
	}
	if fold {
		r = unicode.ToLower(r)
	}
	matched, first := false, true
	for len(p) > 0 && (p[0] != ']' || first) {
		first = false
		if p[0] == '\\' && escape && len(p) > 1 {
			p = p[1:]
		}
		lo, n := utf8.DecodeRuneInString(p)
		p = p[n:]
		hi := lo
		if len(p) > 1 && p[0] == '-' && p[1] != ']' {
			p = p[1:]
			if p[0] == '\\' && escape && len(p) > 1 {
				p = p[1:]
			}
			hi, n = utf8.DecodeRuneInString(p)
			p = p[n:]
		}
		if fold {
			lo, hi = unicode.ToLower(lo), unicode.ToLower(hi)
		}
		if lo <= r && r <= hi {
			matched = true
		}
	}
	if len(p) == 0 {
		return false, "", false
	}
	return matched != neg, p[1:], true
}

// rbBraceExpand is FNM_EXTGLOB's `{a,b}`: every alternative, nested braces expanded too.
func rbBraceExpand(p string, escape bool) []string {
	depth, open := 0, -1
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '\\' && escape:
			i++
		case c == '{':
			if depth == 0 {
				open = i
			}
			depth++
		case c == '}' && depth > 0:
			depth--
			if depth > 0 {
				continue
			}
			var alts []string
			d, from := 0, open+1
			for j := open + 1; j < i; j++ {
				switch p[j] {
				case '\\':
					if escape {
						j++
					}
				case '{':
					d++
				case '}':
					d--
				case ',':
					if d == 0 {
						alts = append(alts, p[from:j])
						from = j + 1
					}
				}
			}
			alts = append(alts, p[from:i])
			out := make([]string, 0, len(alts))
			for _, a := range alts {
				out = append(out, rbBraceExpand(p[:open]+a+p[i+1:], escape)...)
			}
			return out
		}
	}
	return []string{p}
}
