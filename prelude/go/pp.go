//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
//
// PP (decision 113): MRI's PrettyPrint (Oppen's algorithm, prettyprint.rb
// 0.2.0) ported line for line, and pp.rb 0.6.3's layouts for each kind of
// value, chosen by a Go type switch over the closed world instead of
// per-class pretty_print methods.
package prelude

type rbPPGroup struct {
	depth      int
	breakables []*rbPPBreakable
	brk        bool
}

type rbPPText struct {
	objs  []string
	width int
}

type rbPPBreakable struct {
	obj    string
	width  int
	indent int
	group  *rbPPGroup
}

// rbPrettyPrint is PrettyPrint's state; seen is pp.rb's inspect-key set, for cycles.
type rbPrettyPrint struct {
	out         strings.Builder
	maxwidth    int
	outputWidth int
	bufferWidth int
	buffer      []any // *rbPPText or *rbPPBreakable
	groupStack  []*rbPPGroup
	queue       [][]*rbPPGroup
	indent      int
	seen        map[any]bool
}

func newRbPrettyPrint(maxwidth int) *rbPrettyPrint {
	q := &rbPrettyPrint{maxwidth: maxwidth, seen: map[any]bool{}}
	root := &rbPPGroup{}
	q.groupStack = []*rbPPGroup{root}
	q.enq(root)
	return q
}

func (q *rbPrettyPrint) enq(g *rbPPGroup) {
	for g.depth >= len(q.queue) {
		q.queue = append(q.queue, nil)
	}
	q.queue[g.depth] = append(q.queue[g.depth], g)
}

func (q *rbPrettyPrint) deq() *rbPPGroup {
	for d, gs := range q.queue {
		for i := len(gs) - 1; i >= 0; i-- {
			if len(gs[i].breakables) > 0 {
				g := gs[i]
				q.queue[d] = slices.Delete(gs, i, i+1)
				g.brk = true
				return g
			}
		}
		for _, g := range gs {
			g.brk = true
		}
		q.queue[d] = nil
	}
	return nil
}

func (q *rbPrettyPrint) deleteGroup(g *rbPPGroup) {
	if g.depth < len(q.queue) {
		if i := slices.Index(q.queue[g.depth], g); i >= 0 {
			q.queue[g.depth] = slices.Delete(q.queue[g.depth], i, i+1)
		}
	}
}

func (q *rbPrettyPrint) itemOutput(item any) {
	switch d := item.(type) {
	case *rbPPText:
		for _, o := range d.objs {
			q.out.WriteString(o)
		}
		q.outputWidth += d.width
	case *rbPPBreakable:
		d.group.breakables = d.group.breakables[1:]
		if d.group.brk {
			q.out.WriteString("\n" + strings.Repeat(" ", d.indent))
			q.outputWidth = d.indent
			return
		}
		if len(d.group.breakables) == 0 {
			q.deleteGroup(d.group)
		}
		q.out.WriteString(d.obj)
		q.outputWidth += d.width
	}
}

func rbPPWidth(item any) int {
	if t, ok := item.(*rbPPText); ok {
		return t.width
	}
	return item.(*rbPPBreakable).width
}

func (q *rbPrettyPrint) breakOutmostGroups() {
	for q.maxwidth < q.outputWidth+q.bufferWidth {
		g := q.deq()
		if g == nil {
			return
		}
		for len(g.breakables) > 0 {
			data := q.buffer[0]
			q.buffer = q.buffer[1:]
			q.itemOutput(data)
			q.bufferWidth -= rbPPWidth(data)
		}
		for len(q.buffer) > 0 {
			t, ok := q.buffer[0].(*rbPPText)
			if !ok {
				break
			}
			q.buffer = q.buffer[1:]
			q.itemOutput(t)
			q.bufferWidth -= t.width
		}
	}
}

func (q *rbPrettyPrint) text(s string) {
	width := utf8.RuneCountInString(s)
	if len(q.buffer) == 0 {
		q.out.WriteString(s)
		q.outputWidth += width
		return
	}
	t, ok := q.buffer[len(q.buffer)-1].(*rbPPText)
	if !ok {
		t = &rbPPText{}
		q.buffer = append(q.buffer, t)
	}
	t.objs = append(t.objs, s)
	t.width += width
	q.bufferWidth += width
	q.breakOutmostGroups()
}

func (q *rbPrettyPrint) breakable(sep string) {
	g := q.groupStack[len(q.groupStack)-1]
	if g.brk {
		q.flush()
		q.out.WriteString("\n" + strings.Repeat(" ", q.indent))
		q.outputWidth = q.indent
		q.bufferWidth = 0
		return
	}
	width := utf8.RuneCountInString(sep)
	b := &rbPPBreakable{obj: sep, width: width, indent: q.indent, group: g}
	g.breakables = append(g.breakables, b)
	q.buffer = append(q.buffer, b)
	q.bufferWidth += width
	q.breakOutmostGroups()
}

func (q *rbPrettyPrint) group(indent int, open, close string, body func()) {
	q.text(open)
	g := &rbPPGroup{depth: q.groupStack[len(q.groupStack)-1].depth + 1}
	q.groupStack = append(q.groupStack, g)
	q.enq(g)
	func() {
		defer func() {
			q.groupStack = q.groupStack[:len(q.groupStack)-1]
			if len(g.breakables) == 0 {
				q.deleteGroup(g)
			}
		}()
		q.nest(indent, body)
	}()
	q.text(close)
}

func (q *rbPrettyPrint) nest(indent int, body func()) {
	q.indent += indent
	defer func() { q.indent -= indent }()
	body()
}

func (q *rbPrettyPrint) flush() {
	for _, d := range q.buffer {
		q.itemOutput(d)
	}
	q.buffer = nil
	q.bufferWidth = 0
}

func (q *rbPrettyPrint) commaBreakable() {
	q.text(",")
	q.breakable(" ")
}

// seplist calls each for n items with sep between them (comma_breakable by default).
func (q *rbPrettyPrint) seplist(n int, sep func(), each func(int)) {
	if sep == nil {
		sep = q.commaBreakable
	}
	for i := range n {
		if i > 0 {
			sep()
		}
		each(i)
	}
}

// rbPPKey is the identity pp.rb's cycle check keys on: a container's or object's pointer.
func rbPPKey(v any) (any, bool) {
	switch v.(type) {
	case String, Symbol, Integer, Float, Boolean:
		return nil, false
	}
	return v, true
}

// pp is PPMethods#pp: a cycle prints pretty_print_cycle's form, anything else its pretty_print in a group.
func (q *rbPrettyPrint) pp(v any) {
	v = rbUnbox(v)
	key, ref := rbPPKey(v)
	if ref && q.seen[key] {
		q.group(0, "", "", func() { q.ppCycle(v) })
		return
	}
	if ref {
		q.seen[key] = true
		defer delete(q.seen, key)
	}
	q.group(0, "", "", func() { q.prettyPrint(v) })
}

func (q *rbPrettyPrint) ppCycle(v any) {
	switch x := v.(type) {
	case Array_Any:
		q.text(map[bool]string{true: "[]", false: "[...]"}[len(*x._ToAny()) == 0])
	case Hash_Any:
		q.text(map[bool]string{true: "{}", false: "{...}"}[len(x._ToAny().keys) == 0])
	case Set_Any:
		q.text(map[bool]string{true: "Set[]", false: "Set[...]"}[len(x._ToAny().h.keys) == 0])
	case rbPPValueClass:
		q.text("#<" + string(x.__PpKind()) + " " + rbClassName(v) + ":...>")
	default:
		s := strings.TrimSuffix(string(rbObjToS(v)), ">")
		q.group(1, s, ">", func() {
			q.breakable(" ")
			q.text("...")
		})
	}
}

// rbPPValueClass is a Struct or Data class's generated pp support (decision 113).
type rbPPValueClass interface {
	__PpKind() String
	__PpValues() *Array[any]
	Members() *Array[Symbol]
}

// rbPPUser is a class with its own pretty_print(q).
type rbPPUser interface{ PrettyPrint(q *PP) }

func (q *rbPrettyPrint) prettyPrint(v any) {
	switch x := v.(type) {
	case nil:
		q.text("nil")
	case rbPPUser:
		x.PrettyPrint(&PP{q: q})
	case String:
		q.ppString(string(x))
	case Array_Any:
		a := *x._ToAny()
		q.group(1, "[", "]", func() {
			q.seplist(len(a), nil, func(i int) { q.pp(a[i]) })
		})
	case Hash_Any:
		q.ppHash(x._ToAny())
	case Set_Any:
		keys := x._ToAny().h.keys
		q.group(1, "Set[", "]", func() {
			q.seplist(len(keys), nil, func(i int) { q.pp(keys[i]) })
		})
	case Range_Any:
		r := x._ToAny()
		q.pp(r.b)
		q.breakable("")
		q.text(map[bool]string{true: "...", false: ".."}[r.excl])
		q.breakable("")
		if !r.endless {
			q.pp(r.e)
		}
	case rbPPValueClass:
		q.ppValueClass(v, x)
	default:
		if o, ok := v.(interface{ _Ivars() []rbIvar }); ok && rbInspect(v) == rbObjInspect(v) {
			q.ppObject(v, o)
			return
		}
		q.text(string(rbInspect(v)))
	}
}

// ppString is String#pretty_print: a multi-line string is its lines joined by " +".
func (q *rbPrettyPrint) ppString(s string) {
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= 1 {
		q.text(string(rbStringInspect(s)))
		return
	}
	q.group(0, "", "", func() {
		q.seplist(len(lines), func() {
			q.text(" +")
			q.breakable(" ")
		}, func(i int) { q.pp(String(lines[i])) })
	})
}

var rbPPOddSymbol = regexp.MustCompile(`\A:["$@!]|[%&*+\-\/<=>@\]^` + "`" + `|~]\z`)

// ppHash is pp_hash with Ruby 3.4's pair forms (`key: v`, `"odd key": v`, `k => v`).
func (q *rbPrettyPrint) ppHash(h *Hash[any, any]) {
	q.group(1, "{", "}", func() {
		q.seplist(len(h.keys), nil, func(i int) {
			k := h.keys[i]
			v := h.vals[k]
			q.group(0, "", "", func() {
				if sym, ok := rbUnbox(k).(Symbol); ok {
					label := string(sym)
					if rbPPOddSymbol.MatchString(string(rbSymbolInspect(string(sym)))) {
						label = string(rbStringInspect(string(sym)))
					}
					q.text(label + ":")
				} else {
					q.pp(k)
					q.text(" ")
					q.text("=>")
				}
				q.group(1, "", "", func() {
					q.breakable(" ")
					q.pp(v)
				})
			})
		})
	})
}

// ppValueClass is Struct#pretty_print / Data#pretty_print.
func (q *rbPrettyPrint) ppValueClass(v any, x rbPPValueClass) {
	members, values := *x.Members(), *x.__PpValues()
	q.group(1, "#<"+string(x.__PpKind())+" "+rbClassName(v), ">", func() {
		q.seplist(len(members), func() { q.text(",") }, func(i int) {
			q.breakable(" ")
			q.text(string(members[i]) + "=")
			q.group(1, "", "", func() {
				q.breakable("")
				q.pp(values[i])
			})
		})
	})
}

// ppObject is pp_object: the address group with the ivars sorted by name.
func (q *rbPrettyPrint) ppObject(v any, o interface{ _Ivars() []rbIvar }) {
	ivars := slices.Clone(o._Ivars())
	ivars = slices.DeleteFunc(ivars, func(iv rbIvar) bool { return !iv.opt && (iv.val == nil || iv.isNil) })
	slices.SortFunc(ivars, func(a, b rbIvar) int { return strings.Compare(a.name, b.name) })
	s := strings.TrimSuffix(string(rbObjToS(v)), ">")
	q.group(1, s, ">", func() {
		q.seplist(len(ivars), func() { q.text(",") }, func(i int) {
			q.breakable(" ")
			q.text(ivars[i].name + "=")
			q.group(1, "", "", func() {
				q.breakable("")
				q.pp(ivars[i].val)
			})
		})
	})
}

// rbPPWidthFor is PP.width_for: the terminal's width (f nil: none, as for a String), else $COLUMNS, else 80; minus one.
func rbPPWidthFor(f *os.File) int {
	var ws struct{ row, col, x, y uint16 }
	if f != nil {
		errno := syscall.EBADF
		if rc, err := f.SyscallConn(); err == nil { // not f.Fd(), which would switch a pipe to blocking mode
			_ = rc.Control(func(fd uintptr) {
				_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws))) //nolint:gosec // TIOCGWINSZ fills ws
			})
		}
		if errno == 0 && ws.col > 0 {
			return int(ws.col) - 1
		}
	}
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n != 0 {
		return n - 1
	}
	return 79
}

// rbPrettyInspect renders v as PP.pp does, newline included.
func rbPrettyInspect(v any, width int) string {
	q := newRbPrettyPrint(width)
	q.pp(v)
	q.flush()
	return q.out.String() + "\n"
}
