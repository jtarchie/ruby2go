//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
//
// Exception#backtrace (decision 106). A rescue that binds `=> e` records
// the Go call stack's program counters on the exception (rbCaptureBacktrace);
// Exception#backtrace turns them into MRI's lines lazily
// (rbBacktraceFrames), through rbFrameLabels: the table the compiler emits
// from Go function name to Ruby label ('K#m', 'K.s', '<main>'), for the
// functions the pruned program kept.
package prelude

// rbBegin runs a begin/rescue/ensure body. The compiler wraps the func
// literal it makes for one in this call, so a backtrace can tell the
// literal from a Ruby block: its caller is rbBegin. Never inlined, so the
// frame is there to see.
//
//go:noinline
func rbBegin(body func()) { body() }

// rbBeginV is rbBegin for a value-producing body (a constant's initializer).
//
//go:noinline
func rbBeginV[T any](body func() T) T { return body() }

// rbCaptureBacktrace records where r was raised, once: a re-raise keeps the
// original frames, as MRI does. Called inside a rescue's deferred recover,
// where the panicking frames are still on the stack, and before
// rbWrapPanic marks r as rescued (a Go panic is wrapped here first).
func rbCaptureBacktrace(r any) any {
	e, ok := r.(ExceptionI)
	if !ok {
		r = rbWrapPanic(r)
		if e, ok = r.(ExceptionI); !ok {
			return r
		}
	}
	x := e._Exception()
	if x.__pcs != nil {
		return r
	}
	pcs := make([]uintptr, 128)
	n := runtime.Callers(2, pcs)
	x.__pcs = rbPCs{pcs: pcs[:n], rescued: bool(x.__seen)}
	return r
}

// rbPCs is the raw capture: the counters, and whether the exception had
// already passed through a rescue when they were taken (a re-raise, whose
// frames start at the original panic, not the handler's).
type rbPCs struct {
	pcs     []uintptr
	rescued bool
}

// rbBacktrace is Exception#backtrace: nil until the exception was rescued
// with a binding (or set_backtrace), then MRI's lines, built once.
func rbBacktrace(e ExceptionI) **Array[String] { // Array[String]? is **Array (decision 4)
	x := e._Exception()
	if x.__bt != nil {
		return x.__bt
	}
	pcs, ok := x.__pcs.(rbPCs)
	if !ok {
		return nil
	}
	out := &Array[String]{}
	for _, l := range rbBacktraceFrames(pcs.pcs, pcs.rescued) {
		out.s = append(out.s, String(l))
	}
	x.__bt = Ref(out)
	return x.__bt
}

// rbErrorPos is MRI's error_pos: "file:line:in 'name': " for the innermost user frame (a forwarder, "(*T).M", only carries the last //line).
func rbErrorPos(name string) String {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	mod := rbSourceMod()
	for {
		f, more := frames.Next()
		file := strings.TrimPrefix(f.File, mod)
		if strings.HasSuffix(file, ".rb") && !strings.HasPrefix(file, "prelude/") && !strings.Contains(f.Function, ".(") {
			return String(file + ":" + strconv.Itoa(f.Line) + ":in '" + name + "': ")
		}
		if !more {
			return ""
		}
	}
}

// rbFuncKey splits a Go function name into the table's key and how many
// closures deep it is: "main.K_M.func1.1" → "K_M", 2; "main.K_M-range1" →
// "K_M", 1 (a range-over-func body); "main.(*Foo).Bar[...]" → "Foo.Bar", 0.
func rbFuncKey(fn string) (string, int) {
	fn = strings.TrimPrefix(fn, "main.")
	if i := strings.IndexByte(fn, '['); i >= 0 { // generic instantiation: [...] may nest
		depth, j := 0, i
		for ; j < len(fn); j++ {
			switch fn[j] {
			case '[':
				depth++
			case ']':
				depth--
			}
			if depth == 0 {
				break
			}
		}
		fn = fn[:i] + fn[j+1:]
	}
	fn = strings.ReplaceAll(strings.ReplaceAll(fn, "(*", ""), ")", "")
	levels := 0
	for {
		i := strings.LastIndexAny(fn, ".-")
		if i < 0 {
			break
		}
		tail := fn[i+1:]
		switch {
		case fn[i] == '-' && strings.HasPrefix(tail, "range") && rbAllDigits(tail[5:]):
		case fn[i] == '.' && strings.HasPrefix(tail, "func") && rbAllDigits(tail[4:]):
		case fn[i] == '.' && rbAllDigits(tail):
		default:
			return fn, levels
		}
		fn = fn[:i]
		levels++
	}
	return fn, levels
}

func rbAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// rbSourceMod is what -trimpath puts in front of //line file names: this
// file's own path shows it.
var rbSourceMod = sync.OnceValue(func() string {
	_, self, _, _ := runtime.Caller(0)
	return strings.TrimSuffix(self, "prelude/go/backtrace.go")
})

// rbBacktraceFrames renders captured counters as MRI's backtrace lines.
// Frames the compiler or Go made are left out: runtime, prelude Go
// helpers, forwarders (nothing in the table), the begin/rescue func
// literal (its caller is rbBegin) and a deferred handler (its caller is
// the runtime, or the literal it was deferred in). A Ruby-written prelude
// method is a C method to MRI: its label at the caller's file:line, and
// Kernel#raise is no frame at all.
func rbBacktraceFrames(pcs []uintptr, rescued bool) []string {
	var all []runtime.Frame
	frames := runtime.CallersFrames(pcs)
	for {
		f, more := frames.Next()
		all = append(all, f)
		if !more {
			break
		}
	}
	start := 0
	for i, f := range all {
		if f.Function == "runtime.gopanic" {
			start = i + 1
			if !rescued {
				break
			}
		}
	}
	mod := rbSourceMod()
	literals := map[string]bool{} // begin/rescue func literals seen on this stack, by full name
	for i := start; i+1 < len(all); i++ {
		if next, _ := rbFuncKey(all[i+1].Function); next == "rbBegin" || next == "rbBeginV" {
			literals[all[i].Function] = true
		}
	}
	type entry struct {
		label, loc string
		prelude    bool
	}
	var out []entry
	pendingLoc, pendingFor := "", "" // a begin literal's line belongs to the function it is in, which the stack shows at the rbBegin call
	skipTo, dropOnce := "", ""       // past a handler that raised: the frames it unwound, up to its literal, and the function the handler already stands for
	for i := start; i < len(all); i++ {
		f := all[i]
		if skipTo != "" {
			if f.Function != skipTo {
				continue
			}
			skipTo = ""
		}
		if f.Function == dropOnce {
			dropOnce, pendingLoc, pendingFor = "", "", ""
			continue
		}
		if strings.HasPrefix(f.Function, "runtime.") {
			continue
		}
		file := strings.TrimPrefix(f.File, mod)
		loc := file + ":" + strconv.Itoa(f.Line)
		if literals[f.Function] {
			if pendingFor == "" {
				pendingLoc, pendingFor = loc, f.Function[:strings.LastIndexAny(f.Function, ".-")]
			}
			continue
		}
		if f.Function == pendingFor {
			loc, pendingLoc, pendingFor = pendingLoc, "", ""
		}
		base, levels := rbFuncKey(f.Function)
		label, ok := rbFrameLabels[base]
		if !ok {
			continue
		}
		if !strings.HasSuffix(file, ".rb") {
			continue
		}
		prelude := strings.HasPrefix(file, "prelude/")
		if prelude && base == "Kernel_Raise" {
			continue
		}
		if levels > 0 && !prelude {
			next := ""
			if i+1 < len(all) {
				next = all[i+1].Function
			}
			parent := f.Function[:strings.LastIndexAny(f.Function, ".-")]
			switch {
			case strings.HasPrefix(next, "runtime."), literals[next] && next == parent:
				// a rescue or ensure handler raising: it is the method's own frame, so the
				// frames the old panic unwound (up to the handler's literal) and the method's
				// frame below are not shown again
				levels = 0
				if strings.HasPrefix(next, "runtime.") {
					skipTo = parent
				}
				if strings.ContainsAny(parent, ".-") {
					dropOnce = parent[:strings.LastIndexAny(parent, ".-")]
				}
			default:
				for p := parent; strings.ContainsAny(p, ".-"); p = p[:strings.LastIndexAny(p, ".-")] {
					if literals[p] {
						levels--
					}
				}
			}
		}
		switch {
		case prelude:
		case levels == 1:
			label = "block in " + label
		case levels > 1:
			label = fmt.Sprintf("block (%d levels) in %s", levels, label)
		}
		if prelude { // one Ruby-level call of a prelude method is several Go frames (its closures, what it calls): keep where it was entered, the outermost
			for j := len(out) - 1; j >= 0 && out[j].prelude; j-- {
				if out[j].label == label {
					out = slices.Delete(out, j, j+1)
					break
				}
			}
		}
		out = append(out, entry{label: label, loc: loc, prelude: prelude})
	}
	lines := make([]string, 0, len(out))
	loc := ""
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].prelude {
			if loc == "" {
				continue
			}
			out[i].loc = loc
		} else {
			loc = out[i].loc
		}
		lines = append(lines, out[i].loc+":in '"+out[i].label+"'")
	}
	slices.Reverse(lines)
	return lines
}
