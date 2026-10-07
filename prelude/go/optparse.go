//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// rbOptSwitch is one OptionParser#on (decision 101). kind is chosen at
// compile time from the literal switch strings and coercion class.
type rbOptSwitch struct {
	short, long []string // as shown in help: "-v", "--[no-]verbose"
	names       []string // long names without dashes or [no-]
	neg         bool     // --[no-]x
	arg         string   // " NAME", "=FILE", " [PATH]"; "" for a flag
	style       int      // 0 flag, 1 required argument, 2 optional argument
	desc        []string
	kind        string // flag, string, integer, float, array; optional forms end in _opt
	call        func(any)
}

// rbOptItem is a switch or a separator line.
type rbOptItem struct {
	sw  *rbOptSwitch
	sep string
}

func rbNewOptionParser(prog string, banner *String) *OptionParser {
	p := &OptionParser{prog: prog, width: 32, indent: "    "}
	if banner != nil {
		b := string(*banner)
		p.banner = &b
	}
	return p
}

// rbOptSpecStyle splits a switch string's argument part: the style and the
// part shown in help.
func rbOptSpecStyle(rest string) (int, string) {
	switch t := strings.TrimSpace(rest); {
	case t == "":
		return 0, ""
	case strings.Contains(t, "["):
		return 2, rest
	}
	return 1, rest
}

// rbOn adds a switch from on's arguments: strings starting with "-" are
// switches, other strings describe it, class arguments were read at
// compile time.
func (self *OptionParser) rbOn(where int, kind string, specs []any, call func(any)) *OptionParser {
	sw := &rbOptSwitch{kind: kind, call: call}
	for _, a := range specs {
		s, ok := rbUnbox(a).(String)
		if !ok {
			continue
		}
		str := string(s)
		switch {
		case strings.HasPrefix(str, "--"):
			body := str[2:]
			if strings.HasPrefix(body, "[no-]") {
				sw.neg, body = true, body[5:]
			}
			i := strings.IndexAny(body, "= [")
			if i < 0 {
				i = len(body)
			}
			if st, arg := rbOptSpecStyle(body[i:]); st > 0 {
				sw.style, sw.arg = st, arg
			}
			sw.names = append(sw.names, body[:i])
			sw.long = append(sw.long, str[:len(str)-len(body)+i])
		case strings.HasPrefix(str, "-") && len(str) > 1:
			if st, arg := rbOptSpecStyle(str[2:]); st > 0 {
				sw.style, sw.arg = st, arg
				if !strings.HasPrefix(arg, " ") && !strings.HasPrefix(arg, "=") {
					sw.arg = " " + arg
				}
			}
			sw.short = append(sw.short, str[:2])
		default:
			sw.desc = append(sw.desc, str)
		}
	}
	if kind == "flag" && sw.style != 0 {
		sw.kind = "string"
	}
	item := rbOptItem{sw: sw}
	switch where {
	case 1:
		self.tail = append(self.tail, item)
	case -1:
		self.head = append([]rbOptItem{item}, self.head...)
	default:
		self.items = append(self.items, item)
	}
	return self
}

func (self *OptionParser) rbSwitches() []*rbOptSwitch {
	var out []*rbOptSwitch
	for _, list := range [][]rbOptItem{self.head, self.items, self.tail} {
		for _, it := range list {
			if it.sw != nil {
				out = append(out, it.sw)
			}
		}
	}
	return out
}

// rbHelp is MRI's OptionParser#help: the banner, then each switch as
// Switch#summarize lays it out (indent, a 32-column switch field, the
// description after one space; a longer switch field gets a line of its
// own), and separators as they are.
func (self *OptionParser) rbHelp() string {
	var b strings.Builder
	if self.banner != nil {
		b.WriteString(*self.banner)
	} else {
		b.WriteString("Usage: " + self.prog + " [options]")
	}
	b.WriteString("\n")
	width := self.width
	for _, list := range [][]rbOptItem{self.head, self.items, self.tail} {
		for _, it := range list {
			if it.sw == nil {
				b.WriteString(it.sep + "\n")
				continue
			}
			for _, l := range it.sw.summarize(width) {
				b.WriteString(self.indent + l + "\n")
			}
		}
	}
	return b.String()
}

// summarize is optparse's Switch#summarize, line for line.
func (sw *rbOptSwitch) summarize(width int) []string {
	maxw := width - 1
	left := []string{strings.Join(sw.short, ", ")}
	right := slices.Clone(sw.desc)
	for _, s := range sw.long {
		l := len(left[len(left)-1]) + len(s)
		if len(left) == 1 && sw.arg != "" {
			l += len(sw.arg)
		}
		if l >= maxw && len(sw.short) > 0 {
			left = append(left, "")
		}
		if left[len(left)-1] == "" {
			left[len(left)-1] += "    " + s
		} else {
			left[len(left)-1] += ", " + s
		}
	}
	if sw.arg != "" {
		if len(left) > 1 {
			a := sw.arg
			if strings.HasPrefix(a, "[=") {
				a = "[" + a[2:]
			} else {
				a = strings.TrimPrefix(a, "=")
			}
			left[0] += a + ","
		} else {
			left[0] += sw.arg
		}
	}
	var out []string
	mlen := 0
	for _, s := range left {
		mlen = max(mlen, len(s))
	}
	for mlen > width && len(left) > 0 {
		l := left[0]
		left = left[1:]
		if len(l) == mlen {
			mlen = 0
			for _, s := range left {
				mlen = max(mlen, len(s))
			}
		}
		if len(l) < width && len(right) > 0 && right[0] != "" {
			l = fmt.Sprintf("%-*s %s", width, l, right[0])
			right = right[1:]
		}
		out = append(out, l)
	}
	for len(left) > 0 || len(right) > 0 {
		var l, r string
		if len(left) > 0 {
			l, left = left[0], left[1:]
		}
		if len(right) > 0 {
			r, right = right[0], right[1:]
		}
		if r != "" {
			l = fmt.Sprintf("%-*s %s", width, l, r)
		}
		out = append(out, l)
	}
	return out
}

func rbOptErr(kind, msg string) any {
	m := Ref(String(msg))
	switch kind {
	case "invalid option":
		return NewOptionParser_InvalidOption(m)
	case "missing argument":
		return NewOptionParser_MissingArgument(m)
	case "invalid argument":
		return NewOptionParser_InvalidArgument(m)
	case "needless argument":
		return NewOptionParser_NeedlessArgument(m)
	case "ambiguous option":
		return NewOptionParser_AmbiguousOption(m)
	}
	return NewOptionParser_ParseError(m)
}

func rbOptFail(kind, arg string) {
	panic(rbOptErr(kind, kind+": "+arg))
}

// rbFindLong finds a long switch by name or unique prefix, as MRI
// completes them; neg reports a matched --no- form.
func (self *OptionParser) rbFindLong(name, shown string) (*rbOptSwitch, bool) {
	type cand struct {
		sw  *rbOptSwitch
		neg bool
	}
	var exact, prefix []cand
	for _, sw := range self.rbSwitches() {
		for _, n := range sw.names {
			forms := []cand{{sw, false}}
			if sw.neg {
				forms = append(forms, cand{sw, true})
			}
			for _, c := range forms {
				full := n
				if c.neg {
					full = "no-" + n
				}
				switch {
				case full == name:
					exact = append(exact, c)
				case strings.HasPrefix(full, name):
					if !slices.Contains(prefix, c) {
						prefix = append(prefix, c)
					}
				}
			}
		}
	}
	switch {
	case len(exact) > 0:
		return exact[0].sw, exact[0].neg
	case len(prefix) == 1:
		return prefix[0].sw, prefix[0].neg
	case len(prefix) > 1:
		rbOptFail("ambiguous option", shown)
	}
	return nil, false
}

func (self *OptionParser) rbFindShort(c string) *rbOptSwitch {
	for _, sw := range self.rbSwitches() {
		if slices.Contains(sw.short, "-"+c) {
			return sw
		}
	}
	return nil
}

// rbConvert applies the switch's coercion; nil val is an absent optional argument.
func (sw *rbOptSwitch) rbConvert(val *string, shown string) any {
	kind, opt := strings.CutSuffix(sw.kind, "_opt")
	if val == nil {
		if opt {
			switch kind {
			case "integer":
				return (*Integer)(nil)
			case "float":
				return (*Float)(nil)
			}
			return (*String)(nil)
		}
		return String("")
	}
	var out any
	func() {
		defer func() {
			if r := recover(); r != nil {
				rbOptFail("invalid argument", shown)
			}
		}()
		switch kind {
		case "integer":
			out = rbStrictInt(String(*val), 0)
		case "float":
			out = rbStrictFloat(String(*val))
		case "array":
			parts := strings.Split(*val, ",")
			a := &Array[String]{s: make([]String, 0, len(parts))}
			for _, s := range parts {
				a.s = append(a.s, String(s))
			}
			out = a
		default:
			out = String(*val)
		}
	}()
	if !opt {
		return out
	}
	switch v := out.(type) {
	case Integer:
		return &v
	case Float:
		return &v
	case String:
		return &v
	}
	return out
}

// rbOptFire runs a switch: its block, and the into: hash.
func (sw *rbOptSwitch) rbOptFire(v any, into any) {
	if into != nil {
		// key and value are computed inside the cases: a program with no Hash type prunes them (decision 49), and locals only they read would be unused
		switch h := rbUnbox(into).(type) {
		case *Hash[Symbol, any]:
			Hash_Op_idxSet(h, sw.rbOptKey(), rbOptStored(v))
		case *Hash[any, any]:
			Hash_Op_idxSet[any, any](h, sw.rbOptKey(), rbOptStored(v))
		default:
			panic(NewTypeError(Ref(String("into: needs a Hash[Symbol, untyped]"))))
		}
	}
	if sw.call != nil {
		sw.call(v)
	}
}

// rbOptKey is the into: key for a switch: its first long name, else its first short letter.
func (sw *rbOptSwitch) rbOptKey() Symbol {
	if len(sw.names) > 0 {
		return Symbol(sw.names[0])
	}
	if len(sw.short) > 0 {
		return Symbol(sw.short[0][1:])
	}
	return ""
}

// rbOptStored is the value into: stores; an absent optional argument stores nil.
func rbOptStored(v any) any {
	switch x := v.(type) {
	case *String:
		return Opt(x)
	case *Integer:
		return Opt(x)
	case *Float:
		return Opt(x)
	}
	return v
}

// rbParse is OptionParser#parse in permute mode (MRI's default): switches
// anywhere, the rest returned in order, everything after "--" untouched.
func (self *OptionParser) rbParse(argv []String, into any) []String {
	var rest []String
	for i := 0; i < len(argv); {
		a := string(argv[i])
		i++
		next := func() (*string, bool) {
			if i < len(argv) {
				s := string(argv[i])
				return &s, true
			}
			return nil, false
		}
		switch {
		case a == "--":
			return append(rest, argv[i:]...)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a[2:], "=")
			sw, neg := self.rbFindLong(name, "--"+name)
			if sw == nil {
				self.rbOfficious(name, a)
				rbOptFail("invalid option", a)
			}
			shown := "--" + name
			switch sw.style {
			case 0:
				if hasVal {
					rbOptFail("needless argument", a)
				}
				sw.rbOptFire(Boolean(!neg), into)
				continue
			case 1:
				if !hasVal {
					n, ok := next()
					if !ok {
						rbOptFail("missing argument", shown)
					}
					i++
					val = *n
					shown += " " + val
				} else {
					shown = a
				}
				sw.rbOptFire(sw.rbConvert(&val, shown), into)
			default:
				var v *string
				if hasVal {
					v, shown = &val, a
				} else if n, ok := next(); ok && !strings.HasPrefix(*n, "-") {
					i++
					v, shown = n, shown+" "+*n
				}
				sw.rbOptFire(sw.rbConvert(v, shown), into)
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for j := 1; j < len(a); j++ {
				c := a[j : j+1]
				sw := self.rbFindShort(c)
				neg := false
				if sw == nil {
					if sw, neg = self.rbFindLong(c, "-"+c); sw == nil {
						self.rbOfficious(c, a)
						rbOptFail("invalid option", "-"+c)
					}
				}
				if sw.style == 0 {
					sw.rbOptFire(Boolean(!neg), into)
					continue
				}
				var v *string
				shown := a[:j+1]
				switch {
				case j+1 < len(a):
					s := a[j+1:]
					v, shown = &s, a
				case sw.style == 1:
					n, ok := next()
					if !ok {
						rbOptFail("missing argument", shown)
					}
					i++
					v, shown = n, shown+" "+*n
				default:
					if n, ok := next(); ok && !strings.HasPrefix(*n, "-") {
						i++
						v, shown = n, shown+" "+*n
					}
				}
				sw.rbOptFire(sw.rbConvert(v, shown), into)
				break
			}
		default:
			rest = append(rest, argv[i-1])
		}
	}
	return rest
}

// rbOfficious handles the switches every parser has: --help prints the
// help and exits, --version the version (MRI's "officious" options).
func (self *OptionParser) rbOfficious(name, arg string) {
	switch {
	case rbAbbrevOf(name, "help"):
		rbWrite(self.rbHelp())
		panic(NewSystemExit(0))
	case rbAbbrevOf(name, "version") && strings.HasPrefix(arg, "--"):
		if self.version == "" {
			fmt.Fprintln(os.Stderr, self.prog+": version unknown")
			panic(NewSystemExit(1))
		}
		rbWrite(self.prog + " " + self.version + "\n")
		panic(NewSystemExit(0))
	}
}

// rbAbbrevOf reports whether name is a non-empty prefix of full (`--he` for --help).
func rbAbbrevOf(name, full string) bool {
	return name != "" && len(name) <= len(full) && full[:len(name)] == name
}
