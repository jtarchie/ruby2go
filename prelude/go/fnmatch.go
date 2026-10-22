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

// rbFnmPath matches segment lists, `**` segments spanning zero or more path segments. Like MRI's fnmatch it keeps only the
// last `**/` as a backtrack point (an earlier one never needs to retry once a later one matched), so it is linear in the
// match steps instead of exponential in the number of `**/`.
func rbFnmPath(ps, ss []string, flags int) bool {
	pi, si, starP, starS := 0, 0, -1, 0
	for pi < len(ps) || si < len(ss) {
		if pi < len(ps) && ps[pi] == "**" && pi+1 < len(ps) { // `**/`: any directories, none starting with `.` unless FNM_DOTMATCH
			pi++
			starP, starS = pi, si
			continue
		}
		if pi < len(ps) && si < len(ss) && rbFnmSeg(ps[pi], ss[si], flags, true) {
			pi, si = pi+1, si+1
			continue
		}
		if starP < 0 || starS >= len(ss) || strings.HasPrefix(ss[starS], ".") && flags&rbFnmDotmatch == 0 {
			return false
		}
		starS++
		pi, si = starP, starS
	}
	return true
}

// rbFnmSeg matches one pattern against one string; start is whether s begins a path (or segment), for the period rule.
// A `*` remembers only its own position (the last star's), as MRI's fnmatch_helper: earlier stars never retry.
func rbFnmSeg(p, s string, flags int, start bool) bool {
	period := start && flags&rbFnmDotmatch == 0 && strings.HasPrefix(s, ".")
	escape := flags&rbFnmNoescape == 0
	fold := flags&rbFnmCasefold != 0
	slash := flags&rbFnmPathname != 0
	star, starP, starS := false, "", ""
	for {
		if len(p) > 0 && p[0] == '*' {
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if period {
				return false
			}
			star, starP, starS = true, p, s
			continue
		}
		if len(p) == 0 && len(s) == 0 {
			return true
		}
		if len(p) > 0 {
			if np, ns, ok := rbFnmStep(p, s, period, slash, escape, fold); ok {
				p, s, period = np, ns, false
				continue
			}
		}
		if !star || len(starS) == 0 || slash && starS[0] == '/' {
			return false
		}
		_, n := utf8.DecodeRuneInString(starS)
		starS = starS[n:]
		p, s = starP, starS
	}
}

// rbFnmStep matches one non-`*` pattern element (`?`, a class or a literal) against the first rune of s.
func rbFnmStep(p, s string, period, slash, escape, fold bool) (string, string, bool) {
	switch p[0] {
	case '?':
		if len(s) == 0 || period || slash && s[0] == '/' {
			return "", "", false
		}
		_, n := utf8.DecodeRuneInString(s)
		return p[1:], s[n:], true
	case '[':
		if len(s) == 0 || period || slash && s[0] == '/' {
			return "", "", false
		}
		r, n := utf8.DecodeRuneInString(s)
		ok, rest, valid := rbFnmClass(p[1:], r, escape, fold)
		if !valid || !ok { // an unclosed `[` matches nothing, as MRI's
			return "", "", false
		}
		return rest, s[n:], true
	}
	if p[0] == '\\' && escape && len(p) > 1 {
		p = p[1:]
	}
	if len(s) == 0 {
		return "", "", false
	}
	pr, pn := utf8.DecodeRuneInString(p)
	sr, sn := utf8.DecodeRuneInString(s)
	if pr != sr && (!fold || unicode.ToLower(pr) != unicode.ToLower(sr)) {
		return "", "", false
	}
	return p[pn:], s[sn:], true
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
