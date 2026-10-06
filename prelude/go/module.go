//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

type rbConst struct {
	name      string
	value     any
	inherited bool // from a superclass; `inherit = false` skips it
}

type rbModule interface {
	_Kind() string // "class" or "module", for messages
	Name() String
}

// rbConstTable is a class object's generated constant table. Only constant reflection asks for
// it, and no generated interface declares it, so the pruner keeps the tables (and every class
// they name) only in programs that reflect (decision 49).
type rbConstTable interface{ _Consts() []rbConst }

func rbConstName(name any) string {
	switch n := name.(type) {
	case Symbol:
		return string(n)
	case String:
		return string(n)
	}
	panic(NewTypeError(Ref(rbInspect(name) + " is not a symbol nor a string")))
}

func rbConstFind(m rbModule, name string, inherit bool) (any, bool) {
	for _, c := range m.(rbConstTable)._Consts() {
		if c.name == name && (inherit || !c.inherited) {
			return c.value, true
		}
	}
	return nil, false
}

// rbConstRead reads a constant that code may reach before main assigns
// it; MRI raises NameError there.
func rbConstRead[T any](set bool, v T, name string) T {
	if !set {
		panic(NewNameError(Ref(String("uninitialized constant " + name))))
	}
	return v
}

// MRI's rule: uppercase first letter, then identifier (or any non-ASCII) characters.
func rbIsConstName(s string) bool {
	for i, r := range s {
		switch {
		case i == 0:
			if !unicode.IsUpper(r) && !unicode.IsTitle(r) {
				return false
			}
		case r < utf8.RuneSelf && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return false
		}
	}
	return s != ""
}

// rbConstResolve walks an "A::B" path from m; the first segment also
// falls back to the top level, as Module#const_get does. Errors are
// raised segment by segment, in MRI's order. On a miss it returns the
// NameError message.
func rbConstResolve(m rbModule, name any, inherit bool) (any, string) {
	var parts []string
	var path string
	switch n := name.(type) {
	case Symbol:
		path = string(n)
		parts = []string{path}
	case String:
		path = string(n)
		rest := path
		if len(rest) > 2 && strings.HasPrefix(rest, "::") {
			rest, m = rest[2:], Object_class
		}
		parts = strings.Split(rest, "::")
	default:
		what := string(rbInspect(name))
		if _, ok := name.(Boolean); !ok && name != nil {
			what = rbClassName(name)
		}
		panic(NewTypeError(Ref(String("no implicit conversion of " + what + " into String"))))
	}
	var val any
	for i, p := range parts {
		if p == "" || strings.Contains(p, ":") || (i == len(parts)-2 && parts[i+1] == "") {
			panic(NewNameError(Ref(String("wrong constant name " + path))))
		}
		if i > 0 {
			next, ok := val.(rbModule)
			if !ok {
				panic(NewTypeError(Ref(String(path + " does not refer to class/module"))))
			}
			m = next
		}
		if !rbIsConstName(p) {
			panic(NewNameError(Ref(String("wrong constant name " + p))))
		}
		v, ok := rbConstFind(m, p, inherit)
		if !ok && i == 0 && inherit {
			v, ok = rbConstFind(Object_class, p, true)
		}
		if !ok {
			if owner := string(m.Name()); owner != "Object" {
				return nil, "uninitialized constant " + owner + "::" + p
			}
			return nil, "uninitialized constant " + p
		}
		val = v
	}
	return val, ""
}

// rbInstanceTest is a class object's generated _IsInstance (decision 76). It is asked for only
// here, not declared by ModuleI, so the pruner keeps the tests only in programs that use them.
type rbInstanceTest interface{ _IsInstance(v any) bool }

// rbIsInstanceOf is Module#===, and is_a? with a class value.
func rbIsInstanceOf(k, v any) bool {
	t, ok := rbUnbox(k).(rbInstanceTest)
	if !ok {
		panic(NewTypeError(Ref(String("class or module required"))))
	}
	return t._IsInstance(v)
}

// rbDescendants is a class object's generated descendant list, in definition order.
func rbDescendants(k any) []any {
	return k.(interface{ _Descendants() []any })._Descendants()
}

// rbClassVarRead is a class variable first assigned in a method, read
// before any assignment ran: MRI's NameError.
func rbClassVarRead[T any](set bool, v T, name, owner string) T {
	if !set {
		panic(NewNameError(Ref(String("uninitialized class variable " + name + " in " + owner))))
	}
	return v
}
