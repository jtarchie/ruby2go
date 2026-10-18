package compiler

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Dynamic dispatch on untyped values, generated from the closed world:
// no reflection. For every method name called on an untyped value there is
// a `rbDynName` dispatcher, and a `DynName(args ...any) any` wrapper on
// every concrete class whose public, non-generic, block-less method has
// that name (`_DynName` when the method is private: only send and
// receiver-less calls reach it). Wrappers check arity, convert arguments to
// the declared types and box the result, reusing the typed call path.

// emitDynamic emits the dispatchers that became needed (dynNeeded) and reports whether there were any: with the
// computed-send switches in play (dynAll) nearly every method is noted, and a program reaches few of them. A wrapper
// body notes what it calls (a default argument calling something dynamically), and a name noted that late is needed
// by what noted it, so this runs to a fixed point.
func (c *Compiler) emitDynamic(reached, selected func(string) bool) bool {
	if c.dynLazy == nil {
		c.noteDyn("<=>") // rbCmp falls back to DynOp_cmp, and which instantiations see untyped values is unknown here
		if c.dynAll {
			for _, name := range c.allMethodNames(!c.dynAllStrict) {
				c.noteDyn(name)
				c.noteRespond(name)
			}
		}
		c.dynLazy = map[string]bool{}
		for _, n := range slices.Concat(c.dynNames, c.respondNames) {
			c.dynLazy[n] = true
		}
	}
	c.dynEvery = c.dynEvery || c.dynAll && (reached("rbSendByName") || reached("rbRespondsByName"))
	if !c.dynEvery && c.callable == nil && selected("_Call") {
		c.callable = c.callableNames()
	}
	var emitted []string
	for progress := true; progress; {
		progress = false
		for i := 0; i < len(c.dynNames); i++ { //nolint:intrange // emitDynName appends what a wrapper body notes, so len is re-read
			if n := c.dynNames[i]; !c.dynOut[n] && c.dynNeeded(n, reached, selected) {
				c.dynOut[n] = true
				c.emitDynName(n)
				emitted, progress = append(emitted, n), true
			}
		}
		for i := 0; i < len(c.respondNames); i++ { //nolint:intrange // same
			if n := c.respondNames[i]; !c.respondOut[n] && c.dynNeeded(n, reached, selected) {
				c.respondOut[n] = true
				c.emitRespond(n)
				emitted, progress = append(emitted, n), true
			}
		}
	}
	for _, name := range emitted {
		c.emitMarkers("_Private"+goMethodName(name), c.privateIn(name))
	}
	c.emitDynGen()
	return len(emitted) > 0
}

// dynPrivKey prefixes a private method's _DynGen key: NUL cannot start a Ruby name, where "_" also starts a public `_foo`.
const dynPrivKey = "\x00"

// dynGenCase is one wrapper body of a generic class, keyed by its Ruby name (dynPrivKey + name when private).
type dynGenCase struct{ key, body string }

// emitDynGen emits this batch's wrappers of each generic class as one method switching on the name. Go compiles
// every method of a generic type for each instantiation, so 120 DynX methods on Array[E] were 120 bodies per
// element type (64_observable: 10.1k of 51k compiled functions); the batch number keeps names unique across
// emitNext rounds, and the dispatchers this batch emitted name it. The pruner keeps a case only while its name's
// dispatcher is kept (visitSwitch), as a wrapper method was kept only while selected (decision 209).
func (c *Compiler) emitDynGen() {
	if len(c.dynGen) == 0 {
		return
	}
	gen := fmt.Sprintf("_DynGen%d", c.dynGenRound)
	c.dynGenRound++
	for _, cls := range c.classList {
		cases := c.dynGen[cls]
		if len(cases) == 0 {
			continue
		}
		c.w("func (self %s) %s(rbDynName string, args []any) (any, bool) {\n\tswitch rbDynName {\n", c.recvType(cls), gen)
		for _, k := range cases {
			c.w("\tcase %q:\n%s", k.key, okReturns(k.body))
		}
		c.w("\tdefault:\n\t}\n\treturn nil, false\n}\n\n") // default: gocritic rejects a one-case switch
	}
	c.dynGen = nil
}

// okReturns rewrites a wrapper body's `return v` (outside closures) to `return v, true`.
func okReturns(body string) string {
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "", "package p\nfunc _() any {\n"+body+"}\n", 0)
	if err != nil {
		panic(fmt.Sprintf("rb2go: wrapper body does not parse: %v\n%s", err, body))
	}
	fd, _ := f.Decls[0].(*ast.FuncDecl)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			n.Results = append(n.Results, ast.NewIdent("true"))
		}
		return true
	})
	var b strings.Builder
	for _, st := range fd.Body.List {
		b.WriteString("\t\t")
		_ = printer.Fprint(&b, fset, st)
		b.WriteString("\n")
	}
	return b.String()
}

// dynNeeded: name was noted by an emitted body, or kept code calls its dispatcher, asks respond_to? of it, or asserts its wrapper (rbCmp's DynOp_cmp, the json generator's DynToJson).
func (c *Compiler) dynNeeded(name string, reached, selected func(string) bool) bool {
	if c.dynEvery || !c.dynLazy[name] || c.callable[name] {
		return true
	}
	gn := goMethodName(name)
	return reached("rbDyn"+gn) || reached("rbResponds"+gn) || reached("rbHas_Private"+gn) || selected("Dyn"+gn) || selected("_Dyn"+gn)
}

// dynWrapped is one Dyn wrapper emitted on a class; own when a user file defines the method.
// goName is "" for a class a shared arm of the dispatcher serves.
type dynWrapped struct {
	name, goName string
	own          bool
}

// callByName: what minitest calls by name, tests and the predicates and operators of
// assert_predicate/assert_operator. Only these, because naming a common method
// (to_s, name) here would keep its Dyn wrapper on every class (decision 49).
func callByName(name string) bool {
	_, op := opNames[name]
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "?") || op
}

// callableNames are what the _Call tables switch over: the callByName methods user code defines on user classes, own or inherited. Only their dispatchers go out, where a computed send needs every one.
func (c *Compiler) callableNames() map[string]bool {
	out := map[string]bool{}
	for _, cls := range c.classList {
		if cls.File == nil || cls.File.isPrelude() || !cls.isStruct() || cls.metaOf != nil || len(cls.TypeParams) > 0 {
			continue
		}
		for _, e := range cls.methodSet() {
			if !e.M.File.isPrelude() && callByName(e.M.Name) {
				out[e.M.Name] = true
			}
		}
	}
	return out
}

// emitCallTables gives each user-defined class _Call(name, args...): send
// over the methods user code defined on it, own or inherited. It is the
// narrow alternative to rbSendByName, whose switch over every method name
// keeps the whole prelude in a build; minitest calls tests through it
// (decision 79). Only rbMtCall asks for _Call, so other programs prune it.
func (c *Compiler) emitCallTables() {
	for _, cls := range c.classList {
		if cls.File == nil || cls.File.isPrelude() || !cls.isStruct() || cls.metaOf != nil || len(cls.TypeParams) > 0 {
			continue
		}
		var cases []dynWrapped
		for _, w := range c.dynWrapped[cls] {
			if w.own && callByName(w.name) {
				cases = append(cases, w)
			}
		}
		c.w("func (self %s) _Call(name string, args ...any) (any, bool) {\n", c.recvType(cls))
		call := func(w dynWrapped) string {
			if w.goName == "" {
				return fmt.Sprintf("rbDyn%s(rbFCall, self, args...)", goMethodName(w.name))
			}
			return "self." + w.goName + "(args...)"
		}
		switch len(cases) {
		case 0:
		case 1: // gocritic rejects a one-case switch
			c.w("\tif name == %q {\n\t\treturn %s, true\n\t}\n", cases[0].name, call(cases[0]))
		default: // Go compiles a string switch to a search, not a chain of compares
			c.w("\tswitch name {\n")
			for _, w := range cases {
				c.w("\tcase %q:\n\t\treturn %s, true\n", w.name, call(w))
			}
			c.w("\t}\n")
		}
		c.w("\treturn nil, false\n}\n\n")
	}
}

// dynEntry is the method a dynamic call of name reaches on cls, if it can
// be called without a block and without type arguments, and whether it is
// private (the dispatchers call method_missing and respond_to_missing?
// whatever their visibility).
func (c *Compiler) dynEntry(cls *Class, name string) (*entry, bool) {
	if cls.IsModule || cls.universal || name == "initialize" {
		return nil, false
	}
	e := cls.lookup(name)
	if e == nil || e.M.generic() && (e.M.Block == nil || !c.dynBlockable(name, e.M)) || e.M.Block != nil && !c.dynBlockable(name, e.M) {
		return nil, false
	}
	if len(cls.TypeParams) > 0 && growsTypeParams(cls, e) {
		return nil, false
	}
	if cls.hashBase && e.Owner != cls && e.Owner.IsModule && slices.Contains(cls.Super.ancestors(), e.Owner) {
		return nil, false // the embedded base's own module method (Pathname's Comparable): its Self is the base, not cls
	}
	return e, e.M.Private && !rubyPrivate[name] && name != "method_missing"
}

// privateIn lists the concrete classes on which name is a private method.
func (c *Compiler) privateIn(name string) []*Class {
	if len(c.classList) != c.privateInN { // asked per dynamic name, twice: 5 CPU-s of brew's gen unmemoized
		c.privateInMemo, c.privateInN = map[string][]*Class{}, len(c.classList)
	}
	if out, ok := c.privateInMemo[name]; ok {
		return out
	}
	var out []*Class
	defer func() { c.privateInMemo[name] = out }()
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal {
			continue
		}
		if e := cls.lookup(name); e != nil && e.M.Private {
			out = append(out, cls)
		}
	}
	return out
}

// growsTypeParams reports whether a generic class's method mentions its
// type parameters other than bare (E, E?), as the class itself
// (Array[E] on Array) or as an Array of one (Array[E] on Set). Wrapping such a method instantiates some other
// generic at a type built from E (Hash[E, Integer], Array[[E, Integer]]),
// whose own methods and wrappers can instantiate a bigger one still: a Go
// instantiation cycle.
func growsTypeParams(cls *Class, e *entry) bool {
	own := func(t TClass) bool {
		if t.C != cls || len(t.Args) != len(cls.TypeParams) {
			return false
		}
		for i, a := range t.Args {
			if v, ok := a.(TVar); !ok || v.Name != cls.TypeParams[i] {
				return false
			}
		}
		return true
	}
	// Set#to_a's Array[E]: an Array of a bare parameter, whose own wrappers already leave out what grows (another
	// class there, Hash[K, V]'s, closes a Go instantiation cycle through its pairs)
	bareArgs := func(t TClass) bool {
		if t.C.Name != "Array" || len(t.Args) != 1 {
			return false
		}
		v, ok := t.Args[0].(TVar)
		return ok && slices.Contains(cls.TypeParams, v.Name)
	}
	var grows func(t Type) bool
	grows = func(t Type) bool {
		switch t := t.(type) {
		case TOpt:
			return grows(t.Elem)
		case TTuple:
			return mentionsVar(t)
		case TClass:
			return !own(t) && !bareArgs(t) && mentionsVar(t)
		case TUnion: // its package-level rbUnion helper cannot name the class's parameters (Set#merge's Array[E] | Set[E])
			return slices.ContainsFunc(t.Members, grows) || mentionsVar(t)
		case TAny, TFunc, TNil, TVar, TVoid: // no class of its own whose wrappers could grow
		}
		return false
	}
	ts := make([]Type, 0, 1+len(e.M.Params))
	ts = append(ts, e.M.Ret)
	for _, p := range e.M.Params {
		ts = append(ts, p.Type)
	}
	for _, t := range ts {
		if grows(subst(t, e.Env)) {
			return true
		}
	}
	return false
}

// mentionsIdent reports whether Go code names any of idents as a whole word.
func mentionsIdent(code string, idents []string) bool {
	return slices.ContainsFunc(idents, func(id string) bool {
		return regexp.MustCompile(`\b` + regexp.QuoteMeta(id) + `\b`).MatchString(code)
	})
}

func mentionsVar(t Type) bool {
	var vars []string
	freeVars(t, &vars)
	return len(vars) > 0
}

// dispatchers are package-level, so a capital or digit that decision 3 merges (`foo_bar`/`fooBar`) collides across classes too
func (c *Compiler) dynGoName(name string) string {
	gn := goMethodName(name)
	if p, ok := c.dynGo[gn]; ok && p != name {
		c.errorf(nil, nil, "`%s` and `%s` both become Go %s, so dynamic calls cannot tell them apart; rename one", p, name, gn)
	}
	c.dynGo[gn] = name
	return gn
}

// emitDynFallback is a dispatcher's tail before NoMethodError: what a Delegator does not define its object answers, as
// its method_missing does (decisions 294, 295; Homebrew's `dependables.values.map(&:to_a)` of Requirements; a static
// call is decision 118's __getobj__ rewrite instead), then the receiver's own method_missing. Every object has
// Kernel#===, so method_missing is never reached for it.
func (c *Compiler) emitDynFallback(name, gn string) {
	if name == "method_missing" || name == "===" {
		return
	}
	if name != "__getobj__" {
		c.w("\tif o, ok := rbDelegateObj(recv); ok {\n\t\treturn rbDyn%s(how, o, args...)\n\t}\n", gn)
	}
	switch name { // Kernel's block-taking methods on a value whose class defines none of its own (a gem class defines `then`, an Array does not)
	case "then", "yield_self":
		c.w("\tif blk, rest := rbPopBlock(args); blk != nil && len(rest) == 0 {\n\t\treturn blk.fn(recv)\n\t}\n")
	case "tap":
		c.w("\tif blk, rest := rbPopBlock(args); blk != nil && len(rest) == 0 {\n\t\tblk.fn(recv)\n\t\treturn recv\n\t}\n")
	}
	delete(c.dynLazy, "method_missing") // the fallback below needs its wrappers
	c.w("\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
	c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(%q)}, args...)...)\n\t}\n", name)
}

// emitDynName emits rbDynName(how, recv, args...): the public wrapper, the
// private one unless the call had a receiver (how is rbCall), then
// method_missing, then NoMethodError. Wrappers whose bodies are the same
// for many classes (an inherited prelude method: `self.ToS()`) become one
// arm of the dispatcher, chosen by class ID, instead of a copy per class.
func (c *Compiler) emitDynName(name string) {
	gn := c.dynGoName(name)
	hidden := len(c.privateIn(name)) > 0
	own, shared := c.dynWrappers(name)
	var genPublic, genPrivate bool
	own = slices.DeleteFunc(own, func(w dynWrapper) bool {
		if seqNames[name] != "" && w.cls.Name == "Array" || hashViewNames[name] != "" && w.cls.Name == "Hash" {
			return true // the view answers it (rbViewCall / rbHashViewCall), once for every element type
		}
		if len(w.cls.TypeParams) == 0 || dynPinned[name] || name == "to_json" || strings.Contains(w.body, "//line") {
			return false
		}
		if c.dynGen == nil {
			c.dynGen = map[*Class][]dynGenCase{}
		}
		key := name
		if w.private {
			key, genPrivate = dynPrivKey+name, true
		} else {
			genPublic = true
		}
		c.dynGen[w.cls] = append(c.dynGen[w.cls], dynGenCase{key: key, body: w.body})
		return true
	})
	gen := fmt.Sprintf("_DynGen%d", c.dynGenRound)
	c.w("func rbDyn%s(how int, recv any, args ...any) any {\n", gn)
	// the arms go first: their class IDs are exact, and a class they serve
	// may still embed its parent's DynX, promoted by Go with the parent as self
	c.emitDynArms(shared, false)
	c.emitDynArms(shared, true)
	c.w("\tif r, ok := recv.(interface{ Dyn%s(...any) any }); ok {\n\t\treturn r.Dyn%s(args...)\n\t}\n", gn, gn)
	if fn := seqNames[name]; fn != "" {
		c.w("\tif s, ok := recv.(rbView); ok {\n\t\tif v, ok := %s(%q, s, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", fn, name)
	}
	if fn := hashViewNames[name]; fn != "" {
		c.w("\tif s, ok := recv.(rbHashView); ok {\n\t\tif v, ok := %s(%q, s, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", fn, name)
	}
	if genPublic {
		c.w("\tif r, ok := recv.(interface{ %s(string, []any) (any, bool) }); ok {\n\t\tif v, ok := r.%s(%q, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", gen, gen, name)
	}
	if name == "call" {
		c.w("\tif r, ok := rbDynProcCall(recv, args); ok {\n\t\treturn r\n\t}\n")
	}
	kernel := c.nilKernelBody(name) // built once: a body that cannot cross `any` warns
	if m := c.classes["NilClass"].Methods[name]; m != nil && m.File != nil && !m.File.isPrelude() {
		// a reopened NilClass answers nil before Object/Kernel, as markerCall does for a typed nil (decision 281)
		c.w("\tif recv == nil {\n\t\treturn (*NilClass)(nil).Dyn%s(args...)\n\t}\n", gn)
	}
	if v, ok := nilConversions[name]; ok {
		c.w("\tif recv == nil {\n\t\trbArity(len(args), 0, 0)\n\t\treturn %s\n\t}\n", v)
	} else if op, ok := nilOps[name]; ok {
		c.w("\tif recv == nil {\n\t\trbArity(len(args), %d, %d)\n\t\treturn %s\n\t}\n", op.min, op.max, op.code)
	} else if kernel != "" {
		c.w("\tif recv == nil {\n\t\tself := recv\n%s\t}\n", kernel)
	}
	if kernel != "" { // a bare Object (Object.new, main) has only Kernel's and Object's methods; so has a Proc, a Go func (decision 241)
		c.w("\tif _, ok := recv.(*Object); ok || rbIsProc(recv) {\n\t\tself := recv\n%s\t}\n", kernel)
	}
	if hidden {
		c.w("\tif r, ok := recv.(interface{ _Dyn%s(...any) any }); ok && how != rbCall {\n\t\treturn r._Dyn%s(args...)\n\t}\n", gn, gn)
	}
	if genPrivate {
		c.w("\tif r, ok := recv.(interface{ %s(string, []any) (any, bool) }); ok && how != rbCall {\n\t\tif v, ok := r.%s(%q, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", gen, gen, dynPrivKey+name)
	}
	c.emitDynFallback(name, gn)
	if hidden {
		c.w("\tif rbHas_Private%s(recv) {\n\t\tpanic(rbPrivateMethod(%q, recv))\n\t}\n", gn, name)
	}
	if name == "===" {
		// Kernel#===: == unless its class defines one.
		// ponytail: a class object held untyped gets ==, not Module#=== (is_a?);
		// needs a generated per-metaclass instance check.
		c.w("\trbArity(len(args), 1, 1)\n\treturn Boolean(rbEq[any](recv, args[0]))\n}\n\n")
	} else {
		c.w("\tpanic(rbNoMethod(%q, recv, how == rbVCall))\n}\n\n", name)
	}
	for _, w := range own {
		c.emitDynWrapper(w.cls, w.e, name, w.private, w.body)
	}
	for _, a := range shared { // _Call reaches a shared arm through the dispatcher
		for _, w := range a.members {
			c.dynWrapped[w.cls] = append(c.dynWrapped[w.cls], dynWrapped{name: name, own: !c.dynGenerated(w.e)})
		}
	}
}

// nilKernelBody is the wrapper body for nil held untyped calling a public Kernel/Object method (`nil.should`): nil has no Go methods to assert.
func (c *Compiler) nilKernelBody(name string) string {
	obj := c.classes["Object"]
	e := obj.lookup(name)
	if e == nil || e.M.Private || e.M.generic() || e.M.Block != nil || name == "initialize" {
		return ""
	}
	return c.dynWrapperBodyOrWarn(obj, e, name)
}

// dynWrapper is one class's wrapper for a name, before emission.
type dynWrapper struct {
	cls     *Class
	e       *entry
	private bool
	body    string
}

// dynArm is a wrapper body shared by the classes in ids: self is recv
// asserted to iface, or recv itself when iface is empty.
type dynArm struct {
	ids     []string
	iface   string
	body    string
	private bool
	members []dynWrapper
}

// seqNames are the whole-array methods a dynamic call answers through the untyped view (decision 236), by the helper
// that answers them: one body for every element type. The block-taking ones (decision 302) were per-shape wrappers,
// each naming its Enumerable chain once per element type (brew: 393 Array shapes).
var seqNames = viewHelpers(map[string][]string{
	"rbViewCall": {"sort", "sort!", "reverse", "reverse!", "compact", "include?", "index", "find_index", "rindex",
		"count", "max", "min", "join", "uniq", "uniq!", "+", "-", "difference", "&", "intersection", "intersect?", "|", "union", "*",
		"delete", "delete_at", "concat", "replace", "fill", "insert", "rotate", "take", "drop", "assoc", "rassoc", "flatten", "flatten!"},
	"rbViewBlockCall": {"each", "each_entry", "each_with_index", "each_index", "reverse_each", "each_slice", "each_cons", "cycle",
		"first", "select", "filter", "find_all", "reject", "select!", "filter!", "keep_if", "reject!", "delete_if", "map!", "collect!",
		"find", "detect", "rfind", "inject", "reduce", "each_with_object", "sort_by", "sort_by!", "max_by", "min_by",
		"take_while", "drop_while", "bsearch", "bsearch_index"},
	"rbViewPredicate": {"all?", "any?", "none?", "one?"},
	"rbViewSum":       {"sum"},
	"rbViewCore": {"length", "size", "empty?", "frozen?", "to_a", "entries", "deconstruct", "itself", "dup", "clear", "[]",
		"slice", "at", "fetch", "fetch_values", "dig", "last", "[]=", "<<", "push", "append", "unshift", "prepend", "pop",
		"shift", "shuffle", "shuffle!", "sample"},
	"rbViewGrep": {"grep", "grep_v"},
})

func viewHelpers(by map[string][]string) map[string]string {
	out := map[string]string{}
	for fn, names := range by {
		for _, n := range names {
			out[n] = fn
		}
	}
	return out
}

// hashViewNames are the whole-hash methods a dynamic call answers through the Hash view (decisions 236, 302), by helper.
var hashViewNames = viewHelpers(map[string][]string{
	"rbHashViewCall": {"merge", "merge!", "update", "except", "slice", "to_h", "compact", "compact!", "flatten", "value?",
		"has_value?", "key", "replace"},
	"rbHashViewBlockCall": {"each", "each_pair", "each_entry", "each_key", "each_value", "each_with_index", "each_slice",
		"each_cons", "cycle", "select", "filter", "reject", "select!", "filter!", "keep_if", "reject!", "delete_if", "count",
		"find", "detect", "inject", "reduce", "each_with_object", "transform_values!"},
	"rbHashViewPredicate": {"all?", "any?", "none?", "one?"},
})

// dynPinned names wrappers Go code asserts for by method (rbCmp's DynOp_cmp,
// the dispatchers' fallbacks): each class keeps its own. (rbToJson asserts
// DynToJson only after ToJson(...any), the signature of every prelude to_json.)
var dynPinned = map[string]bool{"<=>": true, "method_missing": true, "respond_to_missing?": true}

// dynWrappers splits name's wrappers into those emitted per class and the
// arms shared by two or more classes. Growing with definitions, not with
// classes × names, is what keeps a program of many classes buildable
// (decision 85).
func (c *Compiler) dynWrappers(name string) ([]dynWrapper, []*dynArm) {
	var own []dynWrapper
	arms := map[string]*dynArm{}
	var keys []string
	for _, cls := range c.classList {
		e, private := c.dynEntry(cls, name)
		if e == nil {
			continue
		}
		w := dynWrapper{cls: cls, e: e, private: private, body: c.dynWrapperBodyOrWarn(cls, e, name)}
		if w.body == "" {
			continue
		}
		w.body = c.freeCall(cls, e, w.body)
		iface, body, ok := c.dynShareable(w, name)
		if !ok {
			own = append(own, w)
			continue
		}
		key := fmt.Sprintf("%t\x00%s\x00%s", private, iface, body)
		a := arms[key]
		if a == nil {
			a = &dynArm{iface: iface, body: body, private: private}
			arms[key] = a
			keys = append(keys, key)
		}
		a.ids = append(a.ids, strconv.Itoa(c.classID(cls)))
		a.members = append(a.members, w)
	}
	var shared []*dynArm
	for _, k := range keys {
		if a := arms[k]; len(a.members) > 1 {
			shared = append(shared, a)
		} else {
			own = append(own, a.members[0])
		}
	}
	return own, shared
}

// freeCall rewrites a wrapper's forwarder call of a Kernel method (`self.Sleep(`) into the free func over
// `any` (`Kernel_Sleep[any](self, `): one instantiation for every class, and no selector or interface
// literal naming the method, which would keep the forwarder on every reached class (decision 121). Only a
// universal owner: a named constraint's free func (`Module_Name[ModuleI]`) is not the method a metaclass's
// synth `Name()` overrides in Go, and a module constraint (`Comparable_Self[Self]`) needs the concrete class.
func (c *Compiler) freeCall(cls *Class, e *entry, body string) string {
	call := "self." + e.M.GoName + "("
	if !strings.Contains(body, call) || c.isDirectMethod(e.M) {
		return body
	}
	targ := "any"
	if !e.Owner.universal {
		targ = c.ownerIface(cls, e)
		if targ == "" {
			return body
		}
	}
	if c.plainSelf(e.M) {
		targ = ""
	} else {
		targ = "[" + targ + "]"
	}
	body = strings.ReplaceAll(body, call, freeFuncName(e.M)+targ+"(self, ")
	return strings.ReplaceAll(body, "(self, )", "(self)")
}

// ownerIface is the interface a struct class's wrapper instantiates an inherited class body with, as its forwarder
// does (decision 209): the owner's, which every class below it implements, so no forwarder is selected. "" for a
// module (its constraint may name Self), a generic owner, a wrapper on a primitive, or a class object: a metaclass
// answers Module's and Class's methods through Go-level overrides (synth Name, decision 121) that a promoted wrapper
// of its parent's would bypass.
func (c *Compiler) ownerIface(cls *Class, e *entry) string {
	if !cls.isStruct() || len(cls.TypeParams) > 0 || cls.metaOf != nil || e.Owner.IsModule || !e.Owner.isStruct() || len(e.Owner.TypeParams) > 0 || e.Owner.isSubclassOf(c.classes["Module"]) {
		return ""
	}
	return e.Owner.ifaceName()
}

// dynGenerated: e comes from the prelude, or is a metaclass's generated
// new/name/to_s/inspect, not a method a user file defines.
func (c *Compiler) dynGenerated(e *entry) bool {
	return e.M.Kind == kindSynth || e.M.File != nil && e.M.File.isPrelude()
}

var selfIdent = regexp.MustCompile(`\bself\b`)

// re compiles pattern once per compile: dynShareable's per-owner and per-class patterns repeat for every wrapper.
func (c *Compiler) re(pattern string) *regexp.Regexp {
	if r, ok := c.reCache[pattern]; ok {
		return r
	}
	if c.reCache == nil {
		c.reCache = map[string]*regexp.Regexp{}
	}
	r := regexp.MustCompile(pattern)
	c.reCache[pattern] = r
	return r
}

// dynShareable is w's body with nothing particular to its class, and the
// interface its self must satisfy: a struct class (it has a _ClassID) reaching
// a prelude or generated method whose body only calls that method on self, or passes self
// to a universal owner's free func, whose Self is any.
func (c *Compiler) dynShareable(w dynWrapper, name string) (iface, body string, ok bool) {
	cls, e := w.cls, w.e
	if !c.shareableClass(w, name) {
		return "", "", false
	}
	body = w.body
	if e.Owner.universal {
		for _, t := range []string{c.goType(TClass{C: cls}), c.recvType(cls)} {
			body = strings.ReplaceAll(body, "["+t+"](self", "[any](self")
		}
	}
	rest := strings.ReplaceAll(body, "[any](self", "")
	// a plain free func of the owner (its overloads too: Kernel___Sleep0) takes self as any or the owner's interface (decision 210)
	plain := c.re(`\b` + e.Owner.Name + `_\w+\(self\b`)
	if e.Owner.universal {
		rest = plain.ReplaceAllString(rest, "")
	}
	if oi := c.ownerIface(cls, e); oi != "" && (strings.Contains(rest, "["+oi+"](self") || plain.MatchString(rest)) {
		iface, rest = oi, plain.ReplaceAllString(strings.ReplaceAll(rest, "["+oi+"](self", ""), "") // self is recv asserted to the owner's interface
	} else if oi == "" && !e.Owner.universal && !e.Owner.IsModule && e.Owner.isStruct() && len(e.Owner.TypeParams) == 0 && c.plainSelf(e.M) && plain.MatchString(rest) {
		// a class object's body already calls the owner's plain func (Module_ConstGet(self, ...)): the same text serves every metaclass
		iface, rest = e.Owner.ifaceName(), plain.ReplaceAllString(rest, "")
	}
	if mod := e.Owner; iface == "" && mod.IsModule && !mod.universal && len(mod.TypeParams) == 0 && c.dispatchable(mod, e.M) {
		// a dispatched module method's body over the constraint itself (decision 208), the same for every includer
		// (decision 234): self asserted to the constraint
		inst := c.re(`\b` + regexp.QuoteMeta(freeFuncName(e.M)) + `\[[^\]]*\]\(self\b`)
		if inst.MatchString(body) {
			cons := mod.selfName() + "[any]"
			body = inst.ReplaceAllString(body, freeFuncName(e.M)+"["+cons+"](self")
			iface, rest = cons, inst.ReplaceAllString(rest, "")
		}
	}
	call := "self." + e.M.GoName + "("
	if !w.private && iface == "" && strings.Contains(rest, call) {
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(*e, cls)
		ps, ret := c.sig(e.M, env)
		iface = fmt.Sprintf("interface{ %s(%s) %s }", e.M.GoName, ps, ret)
		rest = strings.ReplaceAll(rest, call, "")
	}
	names := c.re(`\b` + regexp.QuoteMeta(cls.Name) + `(I|_Meta|_class)?\b`) // the class's own Go names, not a substring (rbArity holds an A)
	if selfIdent.MatchString(rest) || names.MatchString(body+iface) {
		return "", "", false
	}
	return iface, body, true
}

// shareableClass: a struct class, or a generic class whose wrapper body is a universal owner's naming none of its type
// parameters (the same text for every instantiation, so one arm instead of a case per element type in _DynGenN, #96).
func (c *Compiler) shareableClass(w dynWrapper, name string) bool {
	cls := w.cls
	if dynPinned[name] {
		return false
	}
	if len(cls.TypeParams) > 0 {
		return w.e.Owner.universal && !mentionsIdent(w.body, cls.TypeParams)
	}
	return cls.isStruct()
}

// emitDynArms emits the shared public (or private) arms: a switch on the
// receiver's class ID, so only the classes that reach the method take it.
func (c *Compiler) emitDynArms(arms []*dynArm, private bool) {
	var cases []*dynArm
	for _, a := range arms {
		if a.private == private {
			cases = append(cases, a)
		}
	}
	if len(cases) == 0 {
		return
	}
	cond := ""
	if private {
		cond = " && how != rbCall"
	}
	c.w("\tif id, ok := recv.(interface{ _ClassID() int }); ok%s {\n", cond)
	arm := func(a *dynArm) {
		switch {
		case a.iface != "":
			c.w("\tif self, ok := recv.(%s); ok {\n%s}\n", a.iface, a.body)
		case selfIdent.MatchString(a.body):
			c.w("\tself := recv\n%s", a.body)
		default:
			c.w("%s", a.body)
		}
	}
	c.w("\tswitch id._ClassID() {\n")
	for _, a := range cases {
		c.w("\tcase %s:\n", strings.Join(a.ids, ", "))
		arm(a)
	}
	c.w("\tdefault:\n\t}\n\t}\n") // default: gocritic rejects a one-case switch; Go compiles the cases to a binary search
}

// dynWrapperBodyOrWarn is cls's wrapper body for e, or "" with a warning
// when its types cannot cross `any`: calling it dynamically raises
// NoMethodError.
func (c *Compiler) dynWrapperBodyOrWarn(cls *Class, e *entry, name string) (body string) {
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			c.warn(nil, nil, "%s#%s cannot be called dynamically: %s", cls.RubyName, name, ce.Error())
			body = ""
		}
	}()
	return c.dynWrapperBody(cls, e)
}

// emitDynWrapper emits cls's DynName (_DynName for a private method).
func (c *Compiler) emitDynWrapper(cls *Class, e *entry, name string, private bool, body string) {
	prefix := "Dyn"
	if private {
		prefix = "_Dyn"
	}
	if cls.RubyName == "Boolean" {
		body = c.boolMarkerBody(name, prefix, body)
	}
	c.w("func (self %s) %s%s(args ...any) any {\n%s}\n\n", c.recvType(cls), prefix, goMethodName(name), body)
	c.dynWrapped[cls] = append(c.dynWrapped[cls], dynWrapped{name: name, goName: prefix + goMethodName(name), own: !e.M.File.isPrelude()})
}

// boolMarkerBody is Boolean's wrapper body for name when the program reopens TrueClass or FalseClass with it: true and
// false answer their own class's method, as a static call does (markerCall, decision 241), not Object's (Homebrew's
// `false.present?` through an untyped value was true; decision 291).
func (c *Compiler) boolMarkerBody(name, prefix, body string) string {
	var b strings.Builder
	for _, side := range []struct {
		cls, cond string
	}{{"TrueClass", "self"}, {"FalseClass", "!self"}} {
		k := c.classes[side.cls]
		if k == nil {
			continue
		}
		if m := k.Methods[name]; m != nil && m.File != nil && !m.File.isPrelude() {
			fmt.Fprintf(&b, "\tif %s {\n\t\treturn (*%s)(nil).%s%s(args...)\n\t}\n", side.cond, k.Name, prefix, goMethodName(name))
		}
	}
	return b.String() + body
}

func (c *Compiler) dynWrapperBody(cls *Class, e *entry) string {
	m := e.M
	file := cls.File
	if m.File != nil {
		file = m.File // default arguments are read from the method's source
	}
	f := c.newFctx(file, cls, nil)
	f.lex = m.Scope
	f.locals = map[localKey]*localInfo{}
	f.scope = &scope{vars: map[string]*local{}}
	f.pass = 2
	f.indent = 1
	f.implicitCall = true // the method may be private: method_missing, or one send reaches
	f.plainCalls = cls.isStruct()
	recv := expr{code: "self", typ: cls.instance()}
	if cls.metaOf != nil || cls.isStruct() {
		recv.typ = TClass{C: cls}
		if e.Owner.IsModule && !e.Owner.universal && e.Entry != nil && e.Entry != cls && e.Entry.isStruct() && c.constraintNamesSelf(e.Owner) {
			// the module body is instantiated at its includer, as cls's forwarder is (selfTypeFor): the includer's
			// methods return its interface, which a subclass's Self would not satisfy (decision 217)
			recv.typ = TClass{C: e.Entry}
		}
	}
	env := composeEnv(e.Env, nil)
	for _, p := range cls.TypeParams {
		env[p] = TVar{Name: p}
	}
	for _, p := range m.TypeParams { // a block-taking generic (`Mutex#synchronize`'s T) called dynamically: untyped (decision 243)
		env[p] = TAny{}
	}
	env["Self"] = recv.typ
	cls.containerEnv(env)
	c.emitDynCall(f, e, recv, env, true)
	return f.buf.String()
}

// dynBlockable reports a block-taking method a dynamic call with a block reaches (decision 243): only names some gem
// call site sends with a block get such wrappers, and not iterators (a range loop) or blocks bound to another self.
func (c *Compiler) dynBlockable(name string, m *Method) bool {
	// ponytail: Kernel's (`tap`, `then`, `open`) would get a wrapper on every class (+5,000 functions in brew); a class's
	// own override still gets one
	if m.Owner != nil && m.Owner.universal {
		return false
	}
	return (c.dynBlockNames[name] || c.dynBlockAll) && (!m.Iterator || !m.hasKeywords()) && (!m.Block.Rest || anyRestBlock(m.Block))
}

// anyRestBlock is a gem's unannotated `&block`, `(*untyped) -> untyped`: the dynamic call's rbBlockArg as it is.
func anyRestBlock(b *BlockSig) bool {
	return b.Rest && len(b.Params) == 1 && isAny(b.Params[0]) && isAny(b.Ret)
}

// dynBlockArg is the block a dynamic call passes as its last argument (rbBlockArg), as the method's typed block: each
// value boxed in, its result asserted out; nil when the call passed none and the block is optional (decision 243).
func (c *Compiler) dynBlockArg(f *fctx, m *Method, env map[string]Type) parser.Node {
	if anyRestBlock(m.Block) { // the call's block as it is, a nil Proc when it gave none
		f.emit("var blkFn_ func(...any) any")
		f.emit("if blk_ != nil {")
		f.emit("\tblkFn_ = blk_.fn")
		f.emit("}")
		ft := TFunc{Params: []Type{TAny{}}, Rest: true, Ret: TAny{}}
		return &parser.BlockArgumentNode{Expression: &exprNode{e: expr{code: "blkFn_", typ: ft}}}
	}
	params := make([]string, len(m.Block.Params))
	vals := make([]string, len(m.Block.Params))
	ps := make([]Type, len(m.Block.Params))
	for i, p := range m.Block.Params {
		ps[i] = subst(p, env)
		params[i] = fmt.Sprintf("a%d_ %s", i, c.goType(ps[i]))
		vals[i] = fmt.Sprintf("a%d_", i)
	}
	ret := subst(m.Block.Ret, env)
	args := strings.Join(vals, ", ")
	var self Type
	if m.Block.Self != nil { // a bound block takes self first; only a bound rbBlockArg runs on it (decision 281)
		self = subst(m.Block.Self, env)
		params = append([]string{"self_ " + c.goType(self)}, params...)
		args = "rbBoundArgs(blk_, " + strings.Join(append([]string{"self_"}, vals...), ", ") + ")..."
	}
	body := "blk_.fn(" + args + ")"
	sig := "func(" + strings.Join(params, ", ") + ")"
	switch {
	case isVoid(ret):
	case isAny(ret):
		sig += " any"
		body = "return " + body
	default:
		sig += " " + c.goType(ret)
		body = fmt.Sprintf("return rbAs[%s](%s, %q)", c.goType(ret), body, ret.String())
	}
	ft := TFunc{Params: ps, Ret: ret, Proc: true, Self: self}
	f.emit("blkFn_ := %s {", sig)
	f.emit("\tif blk_ == nil {")
	f.emit("\t\tpanic(NewLocalJumpError(Ref(String(\"no block given (yield)\"))))")
	f.emit("\t}")
	f.emit("\t%s", body)
	f.emit("}")
	if m.Block.Optional { // the method sees no block, as `block_given?` does
		f.emit("if blk_ == nil {\n\tblkFn_ = nil\n}")
	}
	f.emit("_ = blkFn_")
	return &parser.BlockArgumentNode{Expression: &exprNode{e: expr{code: "(&blkFn_)", typ: ft}}} // a Proc value is boxed: .call derefs it
}

// dynCallBlock takes a dynamic call's block off its arguments (decision 243): the method's block, or dropped when it
// takes none, as MRI ignores it (decision 251).
func (c *Compiler) dynCallBlock(f *fctx, m *Method, env map[string]Type) parser.Node {
	if m.Block == nil && m.Kind == kindSynth && m.Name == "new" && m.Owner.metaOf != nil {
		// a synthesized new takes initialize's block (concurrent-ruby's `new(delayed, 1, *args, &block)` built at run
		// time, decision 255): a block dropped here left BlockedTaskPromise's @Task nil
		if init := m.Owner.metaOf.lookup("initialize"); init != nil && init.M.Block != nil {
			f.emit("blk_, args := rbPopBlock(args)")
			return c.dynBlockArg(f, init.M, env)
		}
	}
	if m.Block != nil {
		f.emit("blk_, args := rbPopBlock(args)")
		return c.dynBlockArg(f, m, env)
	}
	if c.dynBlockNames[m.Name] || c.dynBlockAll { // a computed send with a block may reach any name (Homebrew's cask stanzas)
		f.emit("_, args = rbPopBlock(args)")
	}
	return nil
}

// dynIterCall is a dynamic call with a block of an iterator method: a range over it, each yield into the call's block.
// MRI's value is the method's, often the block's (`synchronize { ... }`): the last block value stands in (decision 243).
func (c *Compiler) dynIterCall(f *fctx, e *entry, recv expr, env map[string]Type, nodes []parser.Node) {
	codes, _ := f.genArgs(&parser.NilNode{}, e.M, env, nodes, nil) // fills defaults and rbArgc as a typed call does
	vars := make([]string, len(e.M.Block.Params))
	for i := range vars {
		vars[i] = fmt.Sprintf("y%d_", i)
	}
	call := f.callCode(e, recv, codes, env)
	f.emit("var last_ any")
	f.emit("if blk_ == nil {")
	f.emit("\tpanic(NewLocalJumpError(Ref(String(\"no block given (yield)\"))))")
	f.emit("}")
	f.emit("%s", rangeHead(strings.Join(vars, ", "), call))
	f.emit("\tlast_ = blk_.fn(%s)", strings.Join(vars, ", "))
	f.emit("}")
	f.emit("return last_")
}

// emitDynCall emits a call to e from an `args ...any` list: MRI's arity check, each argument converted (TypeError), then the typed call, one arm per count of optional arguments; mix adds the numeric coercion arms, which name the receiver `self`.
func (c *Compiler) emitDynCall(f *fctx, e *entry, recv expr, env map[string]Type, mix bool) {
	m := e.M
	if m.postCount() > 0 {
		if c.emitDynPostCall(f, e, recv, env) {
			return
		}
		f.emit("panic(NewArgumentError(Ref(String(%q))))", "rb2go: "+m.String()+" takes post parameters, which a call on an untyped value cannot pass (decision 23)")
		return
	}
	var req, opt int
	var rest *Param
	for i := range m.Params {
		p := &m.Params[i]
		switch {
		case p.Keyword:
		case p.Rest:
			rest = p
		case p.Default != nil:
			opt++
		default:
			req++
		}
	}
	blk := c.dynCallBlock(f, m, env)
	var kwh *exprNode // the call's keywords, passed on as **kwh (#51)
	if m.hasKeywords() {
		f.emit("kwh, args := rbDynKw(args, %d)", req)
		kwh = &exprNode{e: expr{code: "kwh", typ: TClass{C: c.classes["Hash"], Args: []Type{TClass{C: c.classes["Symbol"]}, TAny{}}}}}
	}
	maxArgs := req + opt
	if rest != nil {
		maxArgs = -1
	}
	c.dynWideClassOverloads(f, e, recv, req, maxArgs)
	c.dynArityOverloads(f, e, recv, env, req, maxArgs)
	f.emit("rbArity(len(args), %d, %d)", req, maxArgs)
	if mix && rest == nil && opt == 0 {
		c.dynTwinsAndNumbers(f, e, recv, env)
	}
	if rest == nil && opt == 0 {
		c.dynClassOverloads(f, e, recv)
	}
	if m.Name == "<=>" && len(m.Params) == 1 && req == 1 {
		if t, ok := subst(m.Params[0].Type, env).(TClass); ok { // MRI's <=> answers nil for an incomparable argument
			f.emit("if _, ok := rbConv[%s](args[0]); !ok {", c.goType(t))
			f.emit("\treturn nil")
			f.emit("}")
		}
	}
	call := func(k int, withRest bool) {
		nodes := make([]parser.Node, 0, k+1)
		for i := range k {
			t := subst(m.Params[i].Type, env)
			nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(t, i), typ: t}})
		}
		if withRest {
			t := subst(rest.Type, env)
			code := fmt.Sprintf("rbRest[%s](args, %d, %q)...", c.goType(t), k, t.String())
			if isAny(t) {
				code = fmt.Sprintf("args[%d:]...", k) // untyped takes anything, nil included
			}
			nodes = append(nodes, &exprNode{e: expr{code: code, typ: t}})
		}
		if kwh != nil {
			nodes = append(nodes, &parser.KeywordHashNode{Elements: []parser.Node{&parser.AssocSplatNode{Value: kwh}}})
		}
		if m.Iterator && blk != nil {
			c.dynIterCall(f, e, recv, env, nodes)
			return
		}
		res := f.callEntry(&parser.NilNode{}, e, recv, nodes, blk)
		if isVoid(res.typ) {
			if res.code != "" {
				f.emit("%s", res.code)
			}
			if !res.noreturn {
				f.emit("return nil")
			}
			return
		}
		f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
	}
	if opt == 0 {
		call(req, rest != nil)
		return
	}
	f.emit("switch len(args) {")
	for k := req; k < req+opt; k++ {
		f.emit("case %d:", k)
		f.indent++
		call(k, false)
		f.indent--
	}
	f.emit("default:")
	f.indent++
	call(req+opt, rest != nil)
	f.indent--
	f.emit("}")
}

// dynArityOverloads routes an argument count the method does not take to its decision-12 `__<name>_<count>`, as a typed call does.
func (c *Compiler) dynArityOverloads(f *fctx, e *entry, recv expr, env map[string]Type, req, maxArgs int) {
	m, cls := e.M, classOf(recv.typ)
	if cls == nil || strings.HasPrefix(m.Name, "__") {
		return
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	for _, x := range cls.methodSet() {
		k, ok := strings.CutPrefix(x.M.Name, prefix)
		n, err := strconv.Atoi(k)
		tw := x.M
		if !ok || err != nil || n >= req && (maxArgs < 0 || n <= maxArgs) || tw.generic() || tw.Block != nil || len(tw.Params) != n || tw.hasKeywords() {
			continue
		}
		c.resolveMethod(tw)
		if slices.ContainsFunc(tw.Params, func(p Param) bool { return p.Rest || p.Default != nil }) {
			continue
		}
		envT := maps.Clone(env) // the overload's owner params (Enumerable's E) in the class's terms
		for k, v := range x.Env {
			envT[k] = subst(v, env)
		}
		nodes := make([]parser.Node, n)
		for i := range n {
			t := subst(tw.Params[i].Type, envT)
			nodes[i] = &exprNode{e: expr{code: c.dynArg(t, i), typ: t}}
		}
		f.emit("if len(args) == %d {", n)
		f.indent++
		res := f.callEntry(&parser.NilNode{}, cls.lookup(tw.Name), recv, nodes, nil)
		if isVoid(res.typ) {
			f.emit("%s", res.code)
			f.emit("return nil")
		} else {
			f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
		}
		f.indent--
		f.emit("}")
	}
}

// dynClassOverloads picks a decision-12 overload by its first argument's run-time class, as a typed call picks it by
// the static one: `x.gsub(/re/, s)` on an untyped String is `__gsub_regexp`, not gsub's (String, String).
func (c *Compiler) dynClassOverloads(f *fctx, e *entry, recv expr) {
	m, cls := e.M, classOf(recv.typ)
	if cls == nil || len(m.Params) == 0 || strings.HasPrefix(m.Name, "__") {
		return
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	for _, x := range cls.methodSet() {
		tw := x.M
		if !strings.HasPrefix(tw.Name, prefix) || tw.generic() || tw.Block != nil || len(tw.Params) != len(m.Params) {
			continue
		}
		c.resolveMethod(tw)
		env := composeEnv(x.Env, nil)
		cls.containerEnv(env) // a Set subclass's E is untyped
		t, ok := subst(tw.Params[0].Type, env).(TClass)
		if !ok || typeEq(t, subst(m.Params[0].Type, env)) {
			continue
		}
		te := cls.lookup(tw.Name)
		f.emit("if x, ok := args[0].(%s); ok {", c.goType(t))
		f.indent++
		nodes := []parser.Node{&exprNode{e: expr{code: "x", typ: t}}}
		for i := 1; i < len(tw.Params); i++ {
			pt := tw.Params[i].Type
			nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(pt, i), typ: pt}})
		}
		res := f.callEntry(&parser.NilNode{}, te, recv, nodes, nil)
		f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
		f.indent--
		f.emit("}")
	}
}

// dynTwinsAndNumbers picks a twin by the argument's class (`xs * ","` is
// Array#*'s String twin) and, on a number, mixes the numeric classes.
func (c *Compiler) dynTwinsAndNumbers(f *fctx, e *entry, recv expr, env map[string]Type) {
	cls := classOf(recv.typ)
	if cls == nil {
		return
	}
	if numLevel(cls) < 0 {
		c.dynNumericTwins(f, cls, e, recv)
		return
	}
	c.dynNumericTwins(f, cls, e, expr{code: "self", typ: TClass{C: cls}})
	c.dynNumericMix(f, e, env)
	c.dynNumericTower(f, cls, e, env)
}

// dynNumericMix emits a number wrapper's answer to an argument of another
// number class, as numericMix compiles typed calls: Comparable's methods
// run on rbNum, and an Integer's own operator with a Float is redone by
// its Float, whose wrappers widen Integer arguments (rbAs).
func (c *Compiler) dynNumericMix(f *fctx, e *entry, env map[string]Type) {
	m := e.M
	if len(m.Params) == 0 {
		return
	}
	for _, p := range m.Params {
		if numLevel(classOf(subst(p.Type, env))) < 0 {
			return
		}
	}
	cond := "rbNumMixed(self, args)"
	var ret string
	switch {
	case m.Owner == c.classes["Comparable"]:
		args := make([]string, len(m.Params))
		for i := range args {
			args[i] = c.dynArg(TAny{}, i)
		}
		cond, ret = "rbNumOther(self, args)", f.rbNumCall(nil, m, "self", args).code
	case isClass(env["Self"], "Integer") && !slices.ContainsFunc(m.Params, func(p Param) bool { return !isNumeric(subst(p.Type, env)) }):
		if fe, private := c.dynEntry(c.classes["Float"], m.Name); fe == nil || private {
			return
		}
		ret = "Float(self).Dyn" + goMethodName(m.Name) + "(args...)"
	default:
		return
	}
	f.emit("if %s {", cond)
	f.emit("\treturn %s", ret)
	f.emit("}")
}

// dynNumericTwins is decision 12's choice of a `__<op>_<class>` twin by the argument's class, made at run time for a number's one-argument method.
func (c *Compiler) dynNumericTwins(f *fctx, cls *Class, e *entry, recv expr) {
	m := e.M
	if len(m.Params) != 1 || !classTwins(m.Name) {
		return
	}
	open := false
	for _, name := range append(slices.Clone(numTower), "String") {
		k := c.classes[name]
		if k == nil {
			continue
		}
		tw := cls.lookup("__" + overloadBase(m.Name) + "_" + snake(k.RubyName))
		if tw == nil || tw.M.generic() || tw.M.Block != nil || len(tw.M.Params) != 1 {
			continue
		}
		if !open {
			f.emit("switch x := args[0].(type) {") // one type per case: the pruner keeps a case only when its class is kept (no Rational code without a Rational)
			open = true
		}
		t := TClass{C: k}
		f.emit("case %s:", c.goType(t))
		f.indent++
		res := f.callEntry(&parser.NilNode{}, tw, recv, []parser.Node{&exprNode{e: expr{code: "x", typ: t}}}, nil)
		f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
		f.indent--
	}
	if open {
		f.emit("default:") // gocritic rejects a one-case switch
		f.emit("}")
	}
}

// dynNumericTower is numericTower at run time: an argument of another number class the parameter does not take is coerced with self up the tower (rbNumCoerce), and the method sent again.
func (c *Compiler) dynNumericTower(f *fctx, cls *Class, e *entry, env map[string]Type) {
	m := e.M
	if len(m.Params) != 1 || !towerOps[m.Name] || !typeEq(subst(m.Params[0].Type, env), TClass{C: cls}) {
		return
	}
	f.emit("if _, ok := rbConv[%s](args[0]); !ok {", c.goType(TClass{C: cls}))
	gn := "Dyn" + c.dynGoName(m.Name)
	f.emit("\tif x, y, ok := rbNumCoerce(self, args[0]); ok {")
	// x's own wrapper, not the rbDyn dispatcher, which a pinned wrapper (DynOp_cmp) would keep in every program
	f.emit("\t\tif r, ok := x.(interface{ %s(...any) any }); ok {", gn)
	f.emit("\t\t\treturn r.%s(y)", gn)
	f.emit("\t\t}")
	f.emit("\t\tpanic(rbNoMethod(%q, x, false))", m.Name)
	f.emit("\t}")
	f.emit("}")
}

// dynArg converts argument i to type t. Past the first it is read through
// rbArg: gosec's G602 tracks the argument count of each call site, and a
// dispatcher branch for more arguments than one site passes (Logger's
// one-argument `write` against REXML's two) reads past it, though rbArity
// has already ruled that branch out.
func (c *Compiler) dynArg(t Type, i int) string {
	arg := fmt.Sprintf("args[%d]", i)
	if i > 0 {
		arg = fmt.Sprintf("rbArg(args, %d)", i)
	}
	switch t := t.(type) {
	case TAny:
		return arg
	case TOpt:
		return fmt.Sprintf("OptOf[%s](%s, %q)", c.goType(t.Elem), arg, t.Elem.String())
	case TUnion:
		return c.unionFn(t) + "(" + arg + ")"
	case TClass, TFunc, TNil, TTuple, TVar, TVoid: // converted and checked by rbAs below
	}
	return fmt.Sprintf("rbAs[%s](%s, %q)", c.goType(t), arg, t.String())
}

// emitMarkers emits rbHas<marker>(recv), true for the classes given: one
// switch on the class ID, where a marker method was a function per class
// (and per instantiation of a generic one), kept as a stub whenever the
// assertion was (decisions 87, 209). Boolean keeps a method: no value
// answers its ID.
func (c *Compiler) emitMarkers(marker string, classes []*Class) {
	if c.markers[marker] {
		return
	}
	c.markers[marker] = true
	var ids []string
	methods := false
	for _, cls := range classes {
		if cls == c.classes["Boolean"] { // true and false answer TrueClass's and FalseClass's IDs, never Boolean's
			c.w("func (self %s) %s() {}\n\n", c.recvType(cls), marker)
			methods = true
		} else {
			ids = append(ids, strconv.Itoa(c.classID(cls)))
		}
	}
	c.w("func rbHas%s(recv any) bool {\n", marker)
	if methods {
		c.w("\tif _, ok := recv.(interface{ %s() }); ok {\n\t\treturn true\n\t}\n", marker)
	}
	if len(ids) > 0 {
		c.w("\tif id, ok := recv.(interface{ _ClassID() int }); ok {\n")
		c.w("\t\tswitch id._ClassID() {\n\t\tcase %s:\n\t\t\treturn true\n\t\tdefault:\n\t\t}\n\t}\n", strings.Join(ids, ", "))
	}
	c.w("\treturn false\n}\n\n")
}

// emitRespond emits rbRespondsName(recv, priv), backed by a
// marker method on every class that has the public method (with or without
// a block) and, when private methods count, the _PrivateName markers.
func (c *Compiler) emitRespond(name string) {
	gn := c.dynGoName(name)
	var public []*Class
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal || rubyPrivate[name] {
			continue
		}
		if e := cls.lookup(name); e != nil && !e.M.Private {
			public = append(public, cls)
		}
	}
	c.emitMarkers("_Responds"+gn, public)
	c.w("func rbResponds%s(recv any, priv bool) Boolean {\n", gn)
	c.w("\tif rbHas_Responds%s(recv) {\n\t\treturn true\n\t}\n", gn)
	if len(c.privateIn(name)) > 0 {
		c.w("\tif priv && rbHas_Private%s(recv) {\n\t\treturn true\n\t}\n", gn)
	}
	delete(c.dynLazy, "respond_to_missing?") // the fallback below needs its wrappers
	c.w("\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(%q), Boolean(priv))))\n\t}\n", name)
	c.w("\treturn false\n}\n\n")
}

// allMethodNames lists every method name a computed send may name.
func (c *Compiler) allMethodNames(gemOnly bool) []string {
	seen := map[string]bool{}
	lits := c.symbolLiterals()
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal {
			continue
		}
		preludeOnly := gemOnly && cls.File != nil && cls.File.isPrelude() // its names count only as symbol literals in gem or user code (decision 235)
		for _, e := range cls.methodSet() {
			helper := strings.HasPrefix(e.M.Name, "_") && e.M.File != nil && e.M.File.isPrelude() // the views' accessors, _to_any ...
			if e.M.Name != "initialize" && !strings.HasPrefix(e.M.Name, "__") && !helper && (!preludeOnly || lits[e.M.Name]) {
				seen[e.M.Name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

var symbolLiteral = regexp.MustCompile(`(?:^|[^:\w]):([A-Za-z_]\w*[?!=]?|[<>=!~+\-*/%^&|\[\]]+@?)|%i\[([^\]\n]*)\]`)

// symbolLiterals is every symbol written in a gem or user file (`:name`, `&:name`, `%i[a b]`): what a computed send
// there can name without building a string. Over-approximate by regex; a wrong extra name only widens the set.
func (c *Compiler) symbolLiterals() map[string]bool {
	if c.symLits != nil {
		return c.symLits
	}
	c.symLits = map[string]bool{}
	word := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*[?!=]?|[<>=!~+\-*/%^&|\[\]]+@?`)
	for _, f := range c.files {
		if f.isPrelude() {
			continue
		}
		for _, m := range symbolLiteral.FindAllSubmatch(f.Src, -1) {
			for _, w := range word.FindAll(append(m[1], m[2]...), -1) { // `:name`, `&:name` or `%i[a b]` (one group is empty)
				c.symLits[string(w)] = true
			}
		}
	}
	return c.symLits
}

// emitNameSwitches backs send and respond_to? with computed names.
func (c *Compiler) emitNameSwitches() {
	c.w("func rbSendByName(recv any, name string, how int, args ...any) any {\n\tswitch name {\n")
	for _, name := range c.dynNames {
		c.w("\tcase %q:\n\t\treturn rbDyn%s(how, recv, args...)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
	c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(name)}, args...)...)\n\t}\n")
	c.w("\tpanic(rbNoMethod(name, recv, false))\n}\n\n")
	c.w("func rbRespondsByName(recv any, name string, priv bool) Boolean {\n\tswitch name {\n")
	for _, name := range c.respondNames {
		c.w("\tcase %q:\n\t\treturn rbResponds%s(recv, priv)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(name), Boolean(priv))))\n\t}\n")
	c.w("\treturn false\n}\n\n")
}

// emitClassOf emits rbClassOf, `.class` for a value known only at run
// time: a type switch over the @go_type classes, and over each struct
// hierarchy, whose instances answer _ClassOf. nil is Go nil, not an
// instance of a class, so its class object is a bare Class named NilClass.
func (c *Compiler) emitClassOf() {
	if !c.classOf {
		return
	}
	c.w("type rbNilClass struct{ Class }\n\n")
	for _, m := range []string{"Name", "ToS", "Inspect"} {
		c.w("func (*rbNilClass) %s() String { return \"NilClass\" }\n\n", m)
	}
	c.w("var rbNilClassObj = &rbNilClass{}\n\n")
	c.w("func rbClassOf(a any) ClassI {\n\tswitch v := a.(type) {\n\tcase nil:\n\t\treturn rbNilClassObj\n")
	for _, cls := range c.classList {
		switch {
		case cls.universal || cls.IsModule || cls.meta == nil:
		case cls.RubyName == "Boolean":
			c.w("\tcase Boolean:\n\t\treturn rbBoolClass(v)\n")
		case cls.GoType != "" && len(cls.TypeParams) > 0:
			c.w("\tcase %s:\n\t\treturn %s\n", cls.anyName(), classVar(cls))
		case cls.GoType != "":
			c.w("\tcase %s:\n\t\treturn %s\n", c.goType(TClass{C: cls}), classVar(cls))
		case cls.root() == cls && len(cls.TypeParams) == 0:
			// a metaclass's interface lacks the _ClassOf its struct inherits
			c.w("\tcase %s:\n\t\treturn v._ClassOf().(ClassI)\n", c.goType(TClass{C: cls}))
		}
	}
	if obj := c.classes["Object"]; obj.meta != nil { // main and Object.new: universal, so the loop skipped it
		c.w("\tcase *Object:\n\t\treturn %s\n", classVar(obj))
	}
	c.w("\t}\n")
	if proc := c.classes["Proc"]; proc.meta != nil { // a func type has no methods to switch on
		c.w("\tif rbIsProc(a) {\n\t\treturn %s\n\t}\n", classVar(proc))
	}
	// ponytail: generic struct classes fall to NoMethodError; give them an _Any interface to switch on
	c.w("\tpanic(rbNoMethod(\"class\", a, false))\n}\n\n")
}

// emitDelegateObj emits rbDelegateObj(x): the object a Delegator (SimpleDelegator, a LazyObject's evaluated block, ...)
// stands for, through its own __getobj__, so untyped code calls, iterates and splats what MRI's method_missing would
// reach (decision 295). Each delegator class gets a _DelegateObj method; with none, rbDelegateObj answers false.
func (c *Compiler) emitDelegateObj() {
	for _, cls := range c.classList {
		if cls.IsModule || cls.metaOf != nil || !c.isDelegator(cls) {
			continue
		}
		e := cls.lookup("__getobj__")
		if e == nil {
			continue
		}
		body := c.dynWrapperBodyOrWarn(cls, e, "__getobj__")
		if body == "" {
			continue
		}
		c.w("func (self %s) _DelegateObj() any {\n\tvar args []any\n%s}\n\n", c.recvType(cls), body)
	}
	// a tuple in untyped code is an Array there (decision 22): `.map(&:last)` over Homebrew's [type, line] output pairs
	c.w("func rbDelegateObj(x any) (any, bool) {\n\tswitch d := x.(type) {\n\tcase interface{ _DelegateObj() any }:\n\t\treturn d._DelegateObj(), true\n\tcase interface{ _Tuple() *Array[any] }:\n\t\treturn d._Tuple(), true\n\t}\n\treturn nil, false\n}\n\n")
}

// emitDynEach emits rbEachAny (dynEach): an Array's elements, a Hash's pairs, then one case per Go shape of a user or
// gem class's own each; a block of two splits a lone Array value and a block of one gets a pair as an Array, as yield does.
func (c *Compiler) emitDynEach() {
	if !c.dynEach {
		return
	}
	c.w("func rbEachAny(recv any, arity int, blk func(...any)) any {\n")
	c.w("\tif o, ok := rbDelegateObj(recv); ok { // a Delegator's each is its object's (Homebrew's Dependencies)\n\t\trbEachAny(o, arity, blk)\n\t\treturn recv\n\t}\n")
	c.w("\tyield := func(xs ...any) {\n\t\tswitch {\n")
	c.w("\t\tcase arity == 2 && len(xs) == 1:\n\t\t\ta := rbToAry(xs[0]).s\n\t\t\txs = append(a[:len(a):len(a)], nil, nil)[:2]\n")
	c.w("\t\tcase arity == 1 && len(xs) == 2:\n\t\t\txs = []any{&Array[any]{s: xs}}\n\t\t}\n\t\tblk(xs...)\n\t}\n")
	c.w("\tswitch r := recv.(type) {\n\tcase Array_Any:\n\t\tfor _, x := range r._ToAny().s {\n\t\t\tyield(x)\n\t\t}\n")
	c.w("\tcase Hash_Any:\n\t\th := r._ToAny()\n\t\tfor _, k := range h.keys {\n\t\t\tyield(k, h.vals[k])\n\t\t}\n")

	seen := map[string]bool{}
	for _, cls := range c.classList {
		m := cls.Methods["each"]
		if m == nil || m.File == nil || m.File.isPrelude() || m.Block == nil || len(m.Params) > 0 || m.generic() || len(cls.TypeParams) > 0 ||
			len(m.Block.Params) == 0 || len(m.Block.Params) > 2 || !isVoid(m.Block.Ret) || m.Block.Self != nil || m.Block.Rest {
			continue
		}
		if slices.ContainsFunc(m.Block.Params, mentionsVar) {
			continue
		}
		params, ret := c.sig(m, nil)
		shape := "interface{ Each(" + params + ") " + ret + " }"
		if seen[shape] {
			continue
		}
		seen[shape] = true
		vars := []string{"a", "b"}[:len(m.Block.Params)]
		c.w("\tcase %s:\n", shape)
		if m.Iterator {
			c.w("\t\t%s\n\t\t\tyield(%s)\n\t\t}\n", rangeHead(strings.Join(vars, ", "), "r.Each()"), strings.Join(vars, ", "))
			continue
		}
		typed := make([]string, len(vars))
		for i, v := range vars {
			typed[i] = v + " " + c.goType(m.Block.Params[i])
		}
		c.w("\t\tr.Each(func(%s) { yield(%s) })\n", strings.Join(typed, ", "), strings.Join(vars, ", "))
	}
	c.w("\tdefault:\n\t\tpanic(rbNoMethod(\"each\", recv, false))\n\t}\n\treturn recv\n}\n\n")
	// rbElemsAny is an untyped receiver's elements as an Array[untyped] (decision 224): an Array's own, else what its each yields
	c.w("func rbElemsAny(recv any) *Array[any] {\n")
	c.w("\tif o, ok := rbDelegateObj(recv); ok {\n\t\trecv = o\n\t}\n")
	c.w("\tif a, ok := recv.(Array_Any); ok {\n\t\treturn a._ToAny()\n\t}\n")
	c.w("\tout := &Array[any]{}\n\trbEachAny(recv, 1, func(xs ...any) { out.s = append(out.s, xs[0]) })\n\treturn out\n}\n\n")
}

// emitDynPostCall is a Dyn wrapper's call of a method with post parameters (concurrent-ruby's `evaluate_to(*args,
// block)`, decision 254): the leading parameters from the front of args, the posts from the back, the rest between.
// Only required, rest and post parameters, no keywords or block; false for any other shape.
func (c *Compiler) emitDynPostCall(f *fctx, e *entry, recv expr, env map[string]Type) bool {
	m := e.M
	if m.Block != nil || m.hasKeywords() || m.calleeDefaults {
		return false
	}
	var lead, post []Param
	var rest *Param
	for i := range m.Params {
		p := &m.Params[i]
		switch {
		case p.Keyword || p.KwRest || p.Default != nil:
			return false
		case p.Rest:
			rest = p
		case p.Post:
			post = append(post, *p)
		default:
			lead = append(lead, *p)
		}
	}
	maxArgs := len(lead) + len(post)
	if rest != nil {
		maxArgs = -1
	}
	f.emit("rbArity(len(args), %d, %d)", len(lead)+len(post), maxArgs)
	arg := func(t Type, idx string) parser.Node {
		code := "args[" + idx + "]"
		if !isAny(t) {
			code = fmt.Sprintf("rbAs[%s](%s, %q)", c.goType(t), code, t.String())
		}
		return &exprNode{e: expr{code: code, typ: t}}
	}
	nodes := make([]parser.Node, 0, len(m.Params))
	for i, p := range lead {
		nodes = append(nodes, arg(subst(p.Type, env), strconv.Itoa(i)))
	}
	if rest != nil {
		t := subst(rest.Type, env)
		code := fmt.Sprintf("rbRest[%s](args[:len(args)-%d], %d, %q)", c.goType(t), len(post), len(lead), t.String())
		if isAny(t) {
			code = fmt.Sprintf("args[%d:len(args)-%d]", len(lead), len(post))
		}
		code = fmt.Sprintf("(&Array[%s]{s: %s})", c.goType(t), code)
		nodes = append(nodes, &parser.SplatNode{Expression: &exprNode{e: expr{code: code, typ: TClass{C: c.classes["Array"], Args: []Type{t}}}}})
	}
	for j, p := range post {
		nodes = append(nodes, arg(subst(p.Type, env), fmt.Sprintf("len(args)-%d", len(post)-j)))
	}
	res := f.callEntry(&parser.NilNode{}, e, recv, nodes, nil)
	if isVoid(res.typ) {
		if res.code != "" {
			f.emit("%s", res.code)
		}
		if !res.noreturn {
			f.emit("return nil")
		}
		return true
	}
	f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
	return true
}

// dynWideClassOverloads is dynClassOverloads for a twin taking another count of arguments than its method
// (String#[]'s `__idx_regexp(re, capture = 0)` beside `[](i)`, decision 254): chosen by the first argument's class
// and the count, before the method's own arity check would reject the call.
func (c *Compiler) dynWideClassOverloads(f *fctx, e *entry, recv expr, req, maxArgs int) {
	m, cls := e.M, classOf(recv.typ)
	if cls == nil || len(m.Params) == 0 || strings.HasPrefix(m.Name, "__") || maxArgs != req { // a method of fixed arity
		return
	}
	if slices.ContainsFunc(m.Params, func(p Param) bool { return p.Keyword }) {
		return
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	for _, x := range cls.methodSet() {
		tw := x.M
		if !strings.HasPrefix(tw.Name, prefix) || tw.generic() || tw.Block != nil || len(tw.Params) == len(m.Params) || tw.hasKeywords() {
			continue
		}
		c.resolveMethod(tw)
		treq, tmax := arity(tw)
		if tmax < 0 || treq < 1 || treq == req && tmax == req {
			continue
		}
		env := composeEnv(x.Env, nil)
		cls.containerEnv(env)
		t, ok := subst(tw.Params[0].Type, env).(TClass)
		if !ok || typeEq(t, subst(m.Params[0].Type, env)) {
			continue
		}
		te := cls.lookup(tw.Name)
		f.emit("if len(args) >= %d && len(args) <= %d {", treq, tmax)
		f.emit("\tif x, ok := args[0].(%s); ok {", c.goType(t))
		f.indent += 2
		for k := treq; k <= tmax; k++ { // one `if` per count: gocritic rejects a one-case switch
			nodes := []parser.Node{&exprNode{e: expr{code: "x", typ: t}}}
			for i := 1; i < k; i++ {
				pt := subst(tw.Params[i].Type, env)
				nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(pt, i), typ: pt}})
			}
			f.emit("if len(args) == %d {", k)
			res := f.callEntry(&parser.NilNode{}, te, recv, nodes, nil)
			f.emit("\treturn %s", f.coerce(&parser.NilNode{}, res, TAny{}))
			f.emit("}")
		}
		f.indent -= 2
		f.emit("\t}")
		f.emit("}")
	}
}
