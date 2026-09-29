//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbMtAssertish mirrors Minitest::Assertion::RE on a frame's Go function
// name: a Ruby method named assert_*/refute_*/... becomes Assert*/Refute*.
var rbMtAssertish = regexp.MustCompile(`^(Assert|Refute|Flunk|Pass|Fail|Raise|Must|Wont)`)

// rbMtFuncLit strips closure suffixes (.func1, .1) so a block's frame is
// named after its method, as MRI's "block in X#assert_y" label is.
var rbMtFuncLit = regexp.MustCompile(`(\.func\d+|\.\d+)+$`)

// rbMtLocation is Assertion#location from the Go stack, by minitest's
// rule: of the frames in user code (prelude frames stand in for MRI's
// lib/minitest ones, which it filters), the one after the last whose
// method looks like an assertion, else the first.
func rbMtLocation() String {
	pcs := make([]uintptr, 256)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	// -trimpath puts the module path in front of //line names; this file's own name shows it
	_, self, _, _ := runtime.Caller(0)
	mod := strings.TrimSuffix(self, "prelude/go/minitest.go")
	var locs []string
	idx := -1
	deferred, skipTo := "", ""
	for {
		f, more := frames.Next()
		// An Assertion made in a rescue (assert_raises's flunk) runs in a deferred recover, above the frames that panicked, which MRI's stack has already unwound: skip from the panic to the function whose defer this is.
		if f.Function == "runtime.gopanic" {
			skipTo = rbMtFuncLit.ReplaceAllString(deferred, "")
		}
		deferred = f.Function
		if skipTo != "" {
			if f.Function != skipTo {
				if !more {
					break
				}
				continue
			}
			skipTo = ""
		}
		f.File = strings.TrimPrefix(f.File, mod)
		// Ruby bodies are free funcs (Owner_Name); a Go method, "(*T).Name", is a generated forwarder that only inherits the last //line
		if strings.HasSuffix(f.File, ".rb") && !strings.HasPrefix(f.File, "prelude/") && !strings.Contains(f.Function, ".(") {
			name, _, _ := strings.Cut(f.Function, "[")
			name = rbMtFuncLit.ReplaceAllString(name, "")
			name = name[strings.LastIndexAny(name, "._")+1:]
			if rbMtAssertish.MatchString(name) {
				idx = len(locs)
			}
			locs = append(locs, f.File+":"+strconv.Itoa(f.Line))
		}
		if !more {
			break
		}
	}
	switch {
	case idx+1 < len(locs):
		return String(locs[idx+1])
	case len(locs) > 0:
		return String(locs[len(locs)-1])
	}
	return "unknown:-1"
}

// rbMtDiffCmd is Minitest::Assertions.diff: "diff -u" when a diff is on PATH.
var rbMtDiffCmd = sync.OnceValue(func() string {
	p, err := exec.LookPath("diff")
	if err != nil {
		return ""
	}
	return p
})

func rbMtHaveDiff() bool { return rbMtDiffCmd() != "" }

// rbMtDiff runs `diff -u` over two temp files, as minitest does, and
// relabels the headers.
func rbMtDiff(expect, butwas string) String {
	write := func(pattern, s string) string {
		f, err := os.CreateTemp("", pattern)
		if err != nil {
			panic(NewIOError(Ref(String(err.Error()))))
		}
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		_, err = f.WriteString(s)
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
		if err != nil {
			panic(NewIOError(Ref(String(err.Error()))))
		}
		return f.Name()
	}
	a, b := write("expect", expect), write("butwas", butwas)
	defer func() { _, _ = os.Remove(a), os.Remove(b) }()
	out, _ := exec.Command(rbMtDiffCmd(), "-u", a, b).Output() //nolint:gosec,noctx // diff exits 1 on a difference; its path is from LookPath
	res := rbMtSubFirst(`(?m)^--- .+`, string(out), "--- expected")
	res = rbMtSubFirst(`(?m)^\+\+\+ .+`, res, "+++ actual")
	return String(res)
}

// rbMtSubFirst is Ruby's String#sub: only the first match is replaced.
func rbMtSubFirst(pat, s, repl string) string {
	loc := regexp.MustCompile(pat).FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]] + repl + s[loc[1]:]
}

// rbMtPPForDiff is mu_pp_for_diff past mu_pp: where an inspected string
// shows only single (or only double) escaped newlines, they become real
// ones so a diff has lines to compare; then object addresses are masked.
// Ruby's version uses lookbehind, which Go's regexp lacks.
func rbMtPPForDiff(s String) String {
	str := string(s)
	single, double := false, false
	for i := 0; i+1 < len(str); i++ {
		if str[i] != '\\' || str[i+1] != 'n' {
			continue
		}
		prevSlash := i > 0 && str[i-1] == '\\'
		lineStart := i == 0 || str[i-1] == '\n'
		if prevSlash || lineStart {
			double = true
		} else {
			single = true
		}
	}
	if single != double {
		var b strings.Builder
		for i := 0; i < len(str); {
			switch {
			case strings.HasPrefix(str[i:], `\\n`):
				if double {
					b.WriteString("\\n\n")
				} else {
					b.WriteString(`\\n`)
				}
				i += 3
			case strings.HasPrefix(str[i:], `\n`):
				if single {
					b.WriteString("\n")
				} else {
					b.WriteString(`\n`)
				}
				i += 2
			default:
				b.WriteByte(str[i])
				i++
			}
		}
		str = b.String()
	}
	return String(regexp.MustCompile(`:0x[a-fA-F0-9]{4,}`).ReplaceAllLiteralString(str, ":0xXXXXXX"))
}

// rbMtAfterRun holds Minitest.after_run blocks; autorun calls them last-first.
var rbMtAfterRun []func()

// rbMtCall is send for minitest: a user class's generated _Call table, else NoMethodError.
func rbMtCall(recv any, name string, args ...any) any {
	if c, ok := rbUnbox(recv).(interface {
		_Call(name string, args ...any) (any, bool)
	}); ok {
		if v, ok := c._Call(name, args...); ok {
			return v
		}
	}
	panic(rbNoMethod(name, recv, false))
}

// rbProcOrNil is an optional block as a Proc?: a missing block (a nil func) is nil.
func rbProcOrNil(blk func()) **func() {
	if blk == nil {
		return nil
	}
	p := &blk
	return &p
}
