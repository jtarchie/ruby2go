package compiler

import (
	"reflect"
	"testing"
)

// typeWorld is a hand-built hierarchy: Object > Base > {Derived > Leaf, Other}, Object > {Unrelated, Integer, String}.
type typeWorld struct {
	object, base, derived, leaf, other, unrelated, integer, float, str, array, hash, cmp *Class
	metaBase, metaDerived, metaOther                                                     *Class
}

func newTypeWorld() *typeWorld {
	w := &typeWorld{}
	w.object = &Class{Name: "Object", RubyName: "Object", universal: true}
	sub := func(name string, super *Class) *Class {
		return &Class{Name: name, RubyName: name, Super: super}
	}
	w.base = sub("Base", w.object)
	w.derived = sub("Derived", w.base)
	w.leaf = sub("Leaf", w.derived)
	w.other = sub("Other", w.base)
	w.unrelated = sub("Unrelated", w.object)
	w.integer = sub("Integer", w.object)
	w.float = sub("Float", w.object)
	w.str = sub("String", w.object)
	w.array = sub("Array", w.object)
	w.array.TypeParams = []string{"E"}
	w.hash = sub("Hash", w.object)
	w.hash.TypeParams = []string{"K", "V"}
	w.cmp = &Class{Name: "Comparable", RubyName: "Comparable", IsModule: true}
	w.derived.Includes = []Include{{Mod: w.cmp}}
	metaObject := &Class{Name: "Object_Class", metaOf: w.object}
	w.metaBase = &Class{Name: "Base_Class", metaOf: w.base, Super: metaObject}
	w.metaDerived = &Class{Name: "Derived_Class", metaOf: w.derived, Super: w.metaBase}
	w.metaOther = &Class{Name: "Other_Class", metaOf: w.other, Super: w.metaBase}
	return w
}

func cl(c *Class, args ...Type) TClass { return TClass{C: c, Args: args} }
func opt(t Type) TOpt                  { return TOpt{Elem: t} }
func tv(n string) TVar                 { return TVar{Name: n} }
func tup(ts ...Type) TTuple            { return TTuple{Elems: ts} }
func fn(ret Type, ps ...Type) TFunc    { return TFunc{Params: ps, Ret: ret} }

func show(t Type) string {
	if t == nil {
		return "<nil>"
	}
	return t.String()
}

func TestTypeEq(t *testing.T) {
	w := newTypeWorld()
	I, S := cl(w.integer), cl(w.str)
	cases := []struct {
		name string
		a, b Type
		want bool
	}{
		{"same class", I, cl(w.integer), true},
		{"different class", I, S, false},
		{"subclass is not equal", cl(w.derived), cl(w.base), false},
		{"same args", cl(w.array, I), cl(w.array, I), true},
		{"different args", cl(w.array, I), cl(w.array, S), false},
		{"missing args", cl(w.array), cl(w.array, I), false},
		{"nested args", cl(w.hash, S, cl(w.array, I)), cl(w.hash, S, cl(w.array, I)), true},
		{"opt", opt(I), opt(I), true},
		{"opt vs elem", opt(I), I, false},
		{"opt different elem", opt(I), opt(S), false},
		{"tuple", tup(I, S), tup(I, S), true},
		{"tuple order", tup(I, S), tup(S, I), false},
		{"tuple length", tup(I, S), tup(I, S, I), false},
		{"tvar", tv("T"), tv("T"), true},
		{"tvar name", tv("T"), tv("U"), false},
		{"func", fn(S, I), fn(S, I), true},
		{"func ret", fn(S, I), fn(I, I), false},
		{"func arity", fn(S, I), fn(S, I, I), false},
		{"func param", fn(S, I), fn(S, S), false},
		{"any", TAny{}, TAny{}, true},
		{"nil", TNil{}, TNil{}, true},
		{"void", TVoid{}, TVoid{}, true},
		{"nil vs void", TNil{}, TVoid{}, false},
		{"any vs class", TAny{}, I, false},
		{"meta vs class", cl(w.metaBase), cl(w.base), false},
		{"meta", cl(w.metaBase), cl(w.metaBase), true},
		{"go nil vs class", nil, I, false},
		{"tvar vs class", tv("String"), S, false},
		{"tuple vs func", tup(I), fn(I), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := typeEq(c.a, c.b); got != c.want {
				t.Errorf("typeEq(%s, %s) = %v, want %v", show(c.a), show(c.b), got, c.want)
			}
			if got := typeEq(c.b, c.a); got != c.want {
				t.Errorf("typeEq(%s, %s) = %v, want %v (not symmetric)", show(c.b), show(c.a), got, c.want)
			}
		})
	}
}

func TestFits(t *testing.T) {
	w := newTypeWorld()
	I, S := cl(w.integer), cl(w.str)
	cases := []struct {
		name  string
		t, to Type
		want  bool
	}{
		{"same class", I, I, true},
		{"different class", S, I, false},
		{"subclass", cl(w.leaf), cl(w.base), true},
		{"superclass", cl(w.base), cl(w.derived), false},
		{"sibling", cl(w.other), cl(w.derived), false},
		{"includer", cl(w.leaf), cl(w.cmp), true},
		{"metaclass subclass", cl(w.metaDerived), cl(w.metaBase), true},
		{"args", cl(w.hash, S, I), cl(w.hash, S, I), true},
		{"key arg", cl(w.hash, cl(w.unrelated), I), cl(w.hash, S, I), false},
		{"nested arg", cl(w.array, cl(w.array, S)), cl(w.array, cl(w.array, I)), false},
		{"untyped arg", cl(w.array, TAny{}), cl(w.array, I), true},
		{"tvar arg", cl(w.array, I), cl(w.array, tv("E")), true},
		{"opt arg", cl(w.array, opt(S)), cl(w.array, opt(I)), false},
		{"tuple arg", cl(w.array, tup(I, I)), cl(w.array, tup(I, S)), false},
		{"untyped", TAny{}, I, true},
		{"tvar", tv("Self"), I, true},
		{"tuple where class", tup(I, S), cl(w.array, I), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fits(c.t, c.to); got != c.want {
				t.Errorf("fits(%s, %s) = %v, want %v", show(c.t), show(c.to), got, c.want)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	w := newTypeWorld()
	I, S := cl(w.integer), cl(w.str)
	B, D, O := cl(w.base), cl(w.derived), cl(w.other)
	cases := []struct {
		name  string
		a, b  Type
		want  Type // nil: no join
		known string
	}{
		{"nil nil", TNil{}, TNil{}, TNil{}, ""},
		{"nil T gives T?", TNil{}, S, opt(S), ""}, // decision 14
		{"T nil gives T?", S, TNil{}, opt(S), ""},
		{"nil T? stays T?", TNil{}, opt(S), opt(S), ""},
		{"T? nil stays T?", opt(S), TNil{}, opt(S), ""},
		{"nil untyped", TNil{}, TAny{}, TAny{}, ""},
		{"untyped nil", TAny{}, TNil{}, TAny{}, ""},
		{"equal", S, S, S, ""},
		{"equal generic", cl(w.array, I), cl(w.array, I), cl(w.array, I), ""},
		{"T? T", opt(S), S, opt(S), ""},
		{"T T?", S, opt(S), opt(S), ""},
		{"untyped absorbs", S, TAny{}, TAny{}, ""},
		{"untyped absorbs left", TAny{}, S, TAny{}, ""},
		{"subclass to ancestor", D, B, B, ""},
		{"ancestor to subclass", B, D, B, ""},
		{"siblings meet at parent", D, O, B, ""}, // decision 28
		{"siblings meet at parent reversed", O, D, B, ""},
		{"class objects meet at parent metaclass", cl(w.metaDerived), cl(w.metaOther), cl(w.metaBase), ""},
		{"module included", D, cl(w.cmp), cl(w.cmp), ""},
		{"only Object in common", D, cl(w.unrelated), nil, ""},
		{"unrelated primitives", I, S, nil, ""},
		{"Integer and Float mix untyped", I, cl(w.float), TAny{}, ""}, // decision 12, revised
		{"Integer? and Float", opt(I), cl(w.float), TAny{}, ""},       // T? joins T, then untyped absorbs the nil
		{"different type args", cl(w.array, I), cl(w.array, S), nil, ""},
		{"T? U", opt(S), I, nil, ""},
		{"T? U?", opt(S), opt(I), nil, ""},
		{"tvar same", tv("T"), tv("T"), tv("T"), ""},
		{"tvar different", tv("T"), tv("U"), nil, ""},
		{"tuple equal", tup(I, S), tup(I, S), tup(I, S), ""},
		{"leaf to ancestor", cl(w.leaf), B, B, ""},
		{"cousins meet at grandparent", cl(w.leaf), O, B, ""},
		{"T? untyped", opt(S), TAny{}, TAny{}, ""},
		{"void void", TVoid{}, TVoid{}, TVoid{}, ""},
		{"void class", TVoid{}, S, nil, ""},
		{"func equal", fn(S, I), fn(S, I), fn(S, I), ""},
		{"tuple different", tup(I, S), tup(S, I), nil, ""},
		{"generic vs plain class", cl(w.array, I), D, nil, ""},
		{"class object vs instance", cl(w.metaBase), B, nil, ""},
		{"module vs unrelated class", cl(w.cmp), cl(w.unrelated), nil, ""},
		// join(nil, join(D, B)) is B?, so every association order must agree
		// (`x = nil; x = Derived.new if a; x = Base.new if b`).
		{"opt_subclass_to_ancestor", opt(D), B, opt(B), ""},
		{"opt_ancestor_to_subclass", opt(B), D, opt(B), ""},
		{"opt_sibling_to_sibling", opt(D), O, opt(B), ""},
		{"opt_both_sides", opt(D), opt(B), opt(B), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.known != "" {
				known(t, c.known)
			}
			got, ok := join(c.a, c.b)
			if c.want == nil {
				if ok {
					t.Errorf("join(%s, %s) = %s, want no join", show(c.a), show(c.b), show(got))
				}
				return
			}
			if !ok || !typeEq(got, c.want) {
				t.Errorf("join(%s, %s) = %s, %v; want %s", show(c.a), show(c.b), show(got), ok, show(c.want))
			}
		})
	}
}

// Branch order must not change a local's type.
func TestJoinCommutes(t *testing.T) {
	w := newTypeWorld()
	ts := []Type{
		TNil{}, TAny{}, TVoid{}, cl(w.integer), cl(w.float), cl(w.str), opt(cl(w.str)), cl(w.base), cl(w.derived), cl(w.other),
		cl(w.leaf), opt(cl(w.derived)), cl(w.metaDerived), cl(w.metaOther), fn(cl(w.str)),
		cl(w.unrelated), cl(w.cmp), cl(w.array, cl(w.integer)), tv("T"), tup(cl(w.integer), cl(w.str)),
	}
	for _, a := range ts {
		for _, b := range ts {
			ab, okAB := join(a, b)
			ba, okBA := join(b, a)
			if okAB != okBA || (okAB && !typeEq(ab, ba)) {
				t.Errorf("join(%s, %s) = %s, %v but join(%s, %s) = %s, %v",
					show(a), show(b), show(ab), okAB, show(b), show(a), show(ba), okBA)
			}
		}
	}
}

func TestStripOpt(t *testing.T) {
	w := newTypeWorld()
	S := cl(w.str)
	cases := []struct {
		name    string
		in, out Type
	}{
		{"opt", opt(S), S},
		{"plain", S, S},
		{"one level", opt(opt(S)), opt(S)},
		{"nil", TNil{}, TNil{}},
		{"any", TAny{}, TAny{}},
		{"opt tuple", opt(tup(S, S)), tup(S, S)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripOpt(c.in); !typeEq(got, c.out) {
				t.Errorf("stripOpt(%s) = %s, want %s", show(c.in), show(got), show(c.out))
			}
		})
	}
}

func TestSubst(t *testing.T) {
	w := newTypeWorld()
	I, S := cl(w.integer), cl(w.str)
	env := map[string]Type{"E": I, "K": S, "Self": cl(w.base)}
	cases := []struct {
		name    string
		env     map[string]Type
		in, out Type
	}{
		{"bound var", env, tv("E"), I},
		{"unbound var", env, tv("X"), tv("X")},
		{"self", env, tv("Self"), cl(w.base)},
		{"class args", env, cl(w.array, tv("E")), cl(w.array, I)},
		{"nested args", env, cl(w.hash, tv("K"), cl(w.array, tv("E"))), cl(w.hash, S, cl(w.array, I))},
		{"no args", env, S, S},
		{"opt", env, opt(tv("E")), opt(I)},
		{"tuple", env, tup(tv("K"), tv("E"), tv("X")), tup(S, I, tv("X"))},
		{"func", env, fn(tv("K"), tv("E")), fn(S, I)},
		{"empty env", map[string]Type{}, tv("E"), tv("E")},
		{"nil env", nil, cl(w.array, tv("E")), cl(w.array, tv("E"))},
		{"single pass", map[string]Type{"A": tv("B"), "B": I}, tv("A"), tv("B")},
		{"any", env, TAny{}, TAny{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := subst(c.in, c.env); !typeEq(got, c.out) {
				t.Errorf("subst(%s) = %s, want %s", show(c.in), show(got), show(c.out))
			}
		})
	}
}

func TestSubstDoesNotMutate(t *testing.T) {
	w := newTypeWorld()
	in := cl(w.array, tv("E"))
	_ = subst(in, map[string]Type{"E": cl(w.integer)})
	if !typeEq(in, cl(w.array, tv("E"))) {
		t.Errorf("subst mutated its input: %s", in)
	}
}

func TestUnify(t *testing.T) {
	w := newTypeWorld()
	I, S := cl(w.integer), cl(w.str)
	cases := []struct {
		name            string
		env             map[string]Type
		pattern, actual Type
		ok              bool
		want            map[string]Type
	}{
		{"bind var", nil, tv("T"), I, true, map[string]Type{"T": I}},
		{"var vs void", nil, tv("T"), TVoid{}, false, map[string]Type{}},
		{"var vs nil binds nil", nil, tv("T"), TNil{}, true, map[string]Type{"T": TNil{}}},
		{"bound var keeps binding", map[string]Type{"T": I}, tv("T"), S, true, map[string]Type{"T": I}},
		{"nil binding upgrades", map[string]Type{"T": TNil{}}, tv("T"), I, true, map[string]Type{"T": I}},
		{"nil binding stays on nil", map[string]Type{"T": TNil{}}, tv("T"), TNil{}, true, map[string]Type{"T": TNil{}}},
		{"class args", nil, cl(w.array, tv("E")), cl(w.array, I), true, map[string]Type{"E": I}},
		{"hash args", nil, cl(w.hash, tv("K"), tv("V")), cl(w.hash, S, I), true, map[string]Type{"K": S, "V": I}},
		{"class mismatch", nil, cl(w.array, tv("E")), cl(w.hash, S, I), false, map[string]Type{}},
		{"class vs non-class", nil, cl(w.array, tv("E")), TAny{}, false, map[string]Type{}},
		{"subclass matches ancestor", nil, cl(w.base), cl(w.derived), true, map[string]Type{}},
		{"ancestor does not match subclass", nil, cl(w.derived), cl(w.base), false, map[string]Type{}},
		{"subclass matches included module", nil, cl(w.cmp), cl(w.derived), true, map[string]Type{}},
		{"opt vs opt", nil, opt(tv("E")), opt(I), true, map[string]Type{"E": I}},
		{"opt vs nil", nil, opt(tv("E")), TNil{}, true, map[string]Type{}},
		{"opt vs plain", nil, opt(tv("E")), I, true, map[string]Type{"E": I}},
		{"tuple", nil, tup(tv("K"), tv("V")), tup(S, I), true, map[string]Type{"K": S, "V": I}},
		{"tuple length", nil, tup(tv("K"), tv("V")), tup(S), false, map[string]Type{}},
		{"tuple vs class", nil, tup(tv("K"), tv("V")), S, false, map[string]Type{}},
		{"func", nil, fn(tv("U"), tv("E")), fn(S, I), true, map[string]Type{"E": I, "U": S}},
		{"func arity", nil, fn(tv("U"), tv("E")), fn(S, I, I), false, map[string]Type{}},
		{"func vs class", nil, fn(tv("U")), S, false, map[string]Type{}},
		{"repeated var first wins", nil, tup(tv("T"), tv("T")), tup(I, S), true, map[string]Type{"T": I}},
		// unifyAll keeps going after a mismatch so later vars still bind
		{"later vars bind after mismatch", nil, tup(cl(w.array, tv("E")), tv("V")), tup(S, I), false, map[string]Type{"V": I}},
		{"concrete any pattern", nil, TAny{}, I, true, map[string]Type{}},
		{"nil pattern", nil, TNil{}, I, true, map[string]Type{}},
		{"var binds optional", nil, tv("T"), opt(I), true, map[string]Type{"T": opt(I)}},
		// decision 20: T? where T is expected does not unify
		{"class vs optional", nil, S, opt(S), false, map[string]Type{}},
		{"opt pattern takes subclass", nil, opt(cl(w.base)), cl(w.derived), true, map[string]Type{}},
		{"func ret mismatch", nil, fn(I), fn(S), false, map[string]Type{}},
		{"func binds ret after params", nil, fn(tv("R"), I), fn(S, I), true, map[string]Type{"R": S}},
		{"tuple elem mismatch keeps binding", nil, tup(I, tv("V")), tup(S, I), false, map[string]Type{"V": I}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := map[string]Type{}
			for k, v := range c.env {
				env[k] = v
			}
			if got := unify(c.pattern, c.actual, env); got != c.ok {
				t.Errorf("unify(%s, %s) = %v, want %v", show(c.pattern), show(c.actual), got, c.ok)
			}
			if len(env) != len(c.want) {
				t.Errorf("env = %v, want %v", env, c.want)
			}
			for k, v := range c.want {
				if !typeEq(env[k], v) {
					t.Errorf("env[%s] = %s, want %s", k, show(env[k]), show(v))
				}
			}
		})
	}
}

// unify then subst reproduces the actual type.
func TestUnifySubstRoundTrip(t *testing.T) {
	w := newTypeWorld()
	I, S := cl(w.integer), cl(w.str)
	pattern := cl(w.hash, tv("K"), cl(w.array, tup(tv("K"), opt(tv("V")))))
	actual := cl(w.hash, S, cl(w.array, tup(S, opt(I))))
	env := map[string]Type{}
	if !unify(pattern, actual, env) {
		t.Fatalf("unify(%s, %s) failed", pattern, actual)
	}
	if got := subst(pattern, env); !typeEq(got, actual) {
		t.Errorf("subst(unify) = %s, want %s", got, actual)
	}
}

func TestFreeVars(t *testing.T) {
	w := newTypeWorld()
	I := cl(w.integer)
	cases := []struct {
		name string
		in   Type
		want []string
	}{
		{"var", tv("T"), []string{"T"}},
		{"concrete", I, nil},
		{"class args in order", cl(w.hash, tv("K"), cl(w.array, tv("V"))), []string{"K", "V"}},
		{"dedup", tup(tv("T"), opt(tv("T")), tv("U")), []string{"T", "U"}},
		{"func params then ret", fn(tv("R"), tv("A"), tv("B")), []string{"A", "B", "R"}},
		{"opt", opt(tv("E")), []string{"E"}},
		{"any nil void", tup(TAny{}, TNil{}, TVoid{}), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			freeVars(c.in, &got)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("freeVars(%s) = %v, want %v", show(c.in), got, c.want)
			}
		})
	}
	t.Run("appends to existing without duplicates", func(t *testing.T) {
		got := []string{"K"}
		freeVars(cl(w.hash, tv("K"), tv("V")), &got)
		if want := []string{"K", "V"}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

// String() is what diagnostics and rbAs's runtime TypeErrors print, in RBS syntax.
func TestTypeString(t *testing.T) {
	w := newTypeWorld()
	I, S := cl(w.integer), cl(w.str)
	cases := []struct {
		in   Type
		want string
	}{
		{I, "Integer"},
		{cl(w.array, I), "Array[Integer]"},
		{cl(w.hash, S, cl(w.array, opt(I))), "Hash[String, Array[Integer?]]"},
		{cl(w.metaBase), "singleton(Base)"},
		{opt(S), "String?"},
		{opt(tup(I, S)), "[Integer, String]?"},
		{tup(), "[]"},
		{tv("Elem"), "Elem"},
		{fn(TVoid{}, I, S), "^(Integer, String) -> void"},
		{fn(opt(S)), "^() -> String?"},
		{TAny{}, "untyped"},
		{TNil{}, "nil"},
		{TVoid{}, "void"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.in.String(); got != c.want {
				t.Errorf("String() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTypePredicates(t *testing.T) {
	w := newTypeWorld()
	S := cl(w.str)
	for _, c := range []struct {
		name string
		got  bool
		want bool
	}{
		{"isVoid(void)", isVoid(TVoid{}), true},
		{"isVoid(nil type)", isVoid(TNil{}), true},
		{"isVoid(no type)", isVoid(nil), true},
		{"isVoid(untyped)", isVoid(TAny{}), false},
		{"isVoid(String)", isVoid(S), false},
		{"isVoid(nil?)", isVoid(opt(TNil{})), false},
		{"isAny(untyped)", isAny(TAny{}), true},
		{"isAny(String)", isAny(S), false},
		{"isOpt(String?)", isOpt(opt(S)), true},
		{"isOpt(nil)", isOpt(TNil{}), false},
		{"isNil(nil)", isNil(TNil{}), true},
		{"isNil(void)", isNil(TVoid{}), false},
		{"isClass(String)", isClass(S, "String"), true},
		{"isClass(String?)", isClass(opt(S), "String"), false},
		{"isClass(ancestor)", isClass(cl(w.derived), "Base"), false},
		{"isClass(untyped)", isClass(TAny{}, "Object"), false},
		{"classOf(String)", classOf(S) == w.str, true},
		{"classOf(String?)", classOf(opt(S)) == nil, true},
		{"classOf(tuple)", classOf(tup(S)) == nil, true},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestWrapped(t *testing.T) {
	for c, want := range map[string]bool{
		"(f(x))":           true,
		"(*File).Close(f)": false,
		`(String(")"))`:    true,
		`(a)(")")`:         false,
		"x":                false,
		"((x))":            true,
		`(f('('))`:         true,
	} {
		if got := wrapped(c); got != want {
			t.Errorf("wrapped(%q) = %t, want %t", c, got, want)
		}
	}
}
