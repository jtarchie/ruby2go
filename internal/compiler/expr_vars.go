package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// includerIvar: a gem module's ivar its includer's class assigns (elftools' Dynamic reading Segment's
// @offset_from_vma), one field, reached by name (decision 324).
func (f *fctx) includerIvar(name string) bool {
	return f.owner != nil && f.owner.IsModule && f.lenient() && f.c.findIvar(f.owner, name) == nil && f.c.includerClassWrites(f.owner, name)
}

// descendantIvar is a gem class's read of an ivar it never has but a subclass does, itself or through a module it
// includes: kramdown's Parser::Base#add_text(text, tree = @tree), where Parser::Kramdown gets @tree from Html::Parser.
// Read by name, nil on an object without it, as MRI's (decision 330).
func (f *fctx) descendantIvar(name string) bool {
	if f.owner == nil || f.owner.IsModule || !f.owner.isStruct() || !f.lenient() || f.c.findIvar(f.owner, name) != nil {
		return false
	}
	for _, k := range f.c.classList {
		if k != f.owner && !k.IsModule && k.isStruct() && k.isSubclassOf(f.owner) && f.c.findIvar(k, name) != nil {
			return true
		}
	}
	return false
}

// genIvarWrite is an @x assignment: the value converted to the ivar's type, declaring it during discovery.
func (f *fctx) genIvarWrite(n *parser.InstanceVariableWriteNode) expr {
	if f.includerIvar(n.Name) {
		return f.genIncluderIvarWrite(n)
	}
	var exp, ann Type
	if t := f.f.trailingAnnotation(n); t != "" && !isLetFlag(n.Name) {
		ann = f.parseTypeAnn(n, t)
		exp = ann
	} else if iv := f.c.findIvar(f.owner, n.Name); iv != nil {
		exp = iv.Type
	} else if f.discover {
		exp = f.c.openNewType(n.Value) // `@a = Array.new(n, nil)` is open as `@a = []` is
	}
	val := f.genExpr(n.Value, exp)
	decl := val.typ
	if ann != nil {
		val, decl = f.annotatedIvarValue(n, val, ann)
	}
	f.unnarrow("attr:" + strings.TrimPrefix(n.Name, "@"))
	f.unnarrow("ivar:" + n.Name)
	iv := f.ivar(n, n.Name, decl)
	if f.discover && ann == nil && isEmptyLit(n.Value) {
		iv.open = true
	}
	if !fitsValue(val, iv.Type) && f.lenient() && isUnion(val.typ) { // a gem's union into a narrower ivar converts where it lands, as untyped does (decision 320): Homebrew's `@name = owner.name` of a Formula | Cask
		val = expr{code: val.code, typ: TAny{}}
	}
	if !fitsValue(val, iv.Type) && (!f.lenient() || !isAny(val.typ)) { // a gem's `@v = T.let(v, T.untyped)` converts where it lands (decision 225)
		f.errorf(n, "cannot assign %s to %s, which is %s", val.typ, n.Name, iv.Type)
	}
	code := f.ivarCode(iv)
	f.storing++
	f.emit("%s = %s", code, f.coerce(n, val, iv.Type))
	f.storing--
	f.markIvar(iv)
	// the value is what was assigned: `@x = 1` is an Integer even when @x is Integer?
	if isOpt(iv.Type) && !isAny(stripOpt(iv.Type)) && !isOpt(val.typ) && !isNil(val.typ) && !isAny(val.typ) {
		f.narrowIvar(iv) // and reads after it, until a call through self (decision 148)
		return expr{code: "(*" + code + ")", typ: stripOpt(iv.Type), done: true}
	}
	return expr{code: code, typ: iv.Type, done: true}
}

// genIvarRead is an @x read: narrowed, an includer's by name, nil before discovery types it, else the field.
func (f *fctx) genIvarRead(n *parser.InstanceVariableReadNode) expr {
	if v := f.scope.lookup("ivar:" + n.Name); v != nil { // narrowed (decision 148)
		return expr{code: v.goName, typ: v.typ}
	}
	if f.includerIvar(n.Name) || !f.discover && f.descendantIvar(n.Name) {
		return expr{code: "rbIvarGet(" + f.selfCode + ", Symbol(" + strconv.Quote(n.Name) + "))", typ: TAny{}}
	}
	if f.readsUnassigned(n.Name) {
		return expr{code: "nil", typ: TNil{}}
	}
	iv := f.ivar(n, n.Name, nil)
	return expr{code: f.ivarCode(iv), typ: iv.Type}
}

// genIncluderIvarWrite is a module's write of an ivar its includer's class keeps: set by name.
func (f *fctx) genIncluderIvarWrite(n *parser.InstanceVariableWriteNode) expr {
	v := f.genExpr(n.Value, nil)
	tmp := f.newTmp()
	f.emit("var %s any = %s", tmp, f.coerce(n.Value, v, TAny{}))
	f.emit("rbIvarSet(%s, Symbol(%q), %s)", f.selfCode, n.Name, tmp)
	return expr{code: tmp, typ: TAny{}, done: true}
}

// genIvarExpr handles @x reads and writes (and hands @@x to genClassVarExpr).
func (f *fctx) genIvarExpr(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.ClassVariableReadNode, *parser.ClassVariableWriteNode, *parser.ClassVariableOperatorWriteNode, *parser.ClassVariableOrWriteNode:
		return f.genClassVarExpr(n)
	case *parser.InstanceVariableReadNode:
		return f.genIvarRead(n)
	case *parser.InstanceVariableWriteNode:
		return f.genIvarWrite(n)
	case *parser.InstanceVariableOperatorWriteNode:
		iv := f.ivar(n, n.Name, nil)
		code := f.ivarCode(iv)
		cur := expr{code: code, typ: iv.Type}
		if v := f.scope.lookup("ivar:" + n.Name); v != nil { // narrowed: @x is non-nil here (decision 148)
			cur = expr{code: v.goName, typ: v.typ}
		}
		val := f.genOp(n, cur, n.BinaryOperator, n.Value)
		f.unnarrow("ivar:" + n.Name)
		f.emit("%s = %s", code, f.coerce(n, val, iv.Type))
		f.markIvar(iv)
		if isOpt(iv.Type) && !isAny(stripOpt(iv.Type)) && !isOpt(val.typ) && !isNil(val.typ) && !isAny(val.typ) {
			f.narrowIvar(iv)
			return expr{code: "(*" + code + ")", typ: stripOpt(iv.Type), done: true}
		}
		return expr{code: code, typ: iv.Type, done: true}
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

// genClassVarExpr handles @@x: a package variable of the class whose body
// first assigned it, shared by its subclasses (and a module's includers).
func (f *fctx) genClassVarExpr(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.ClassVariableReadNode:
		k := f.classVar(n, n.Name)
		return f.classVarRead(k, n.Name)
	case *parser.ClassVariableWriteNode:
		k := f.classVar(n, n.Name)
		t := f.c.constType(k)
		val := f.genExpr(n.Value, t)
		f.emit("%s = %s", k.GoName, f.coerce(n, val, t))
		f.classVarSet(k)
		return expr{code: k.GoName, typ: t, done: true}
	case *parser.ClassVariableOperatorWriteNode:
		k := f.classVar(n, n.Name)
		t := f.c.constType(k)
		val := f.genOp(n, f.classVarRead(k, n.Name), n.BinaryOperator, n.Value)
		f.emit("%s = %s", k.GoName, f.coerce(n, val, t))
		return expr{code: k.GoName, typ: t, done: true}
	case *parser.ClassVariableOrWriteNode:
		k := f.classVar(n, n.Name)
		t := f.c.constType(k)
		if !k.inMethod {
			return f.genOrAssign(n, expr{code: k.GoName, typ: t}, n.Value)
		}
		// unset reads as nil: assign it then, as MRI's ||= does (or when it holds nil or false)
		cond := "!" + constSet(k)
		if c := f.truthyQuiet(expr{code: k.GoName, typ: t}); c != "" {
			cond += " || !(" + c + ")"
		}
		f.emit("if %s {", cond)
		f.indent++
		val := f.genExpr(n.Value, t)
		f.emit("%s = %s", k.GoName, f.coerce(n, val, t))
		f.classVarSet(k)
		f.indent--
		f.emit("}")
		return expr{code: k.GoName, typ: t, done: true}
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

// classVarRead reads a class variable; one first assigned in a method checks its set flag.
func (f *fctx) classVarRead(k *Const, name string) expr {
	t := f.c.constType(k)
	if !k.inMethod {
		return expr{code: k.GoName, typ: t}
	}
	owner := strings.TrimSuffix(k.RubyName, "::"+name)
	return expr{code: fmt.Sprintf("rbClassVarRead(%s, %s, %q, %q)", constSet(k), k.GoName, name, owner), typ: t}
}

func (f *fctx) classVarSet(k *Const) {
	if k.inMethod {
		f.emit("%s = true", constSet(k))
	}
}

// classVar finds @@name from the code's class: its own, an ancestor's, or an included module's.
func (f *fctx) classVar(n parser.Node, name string) *Const {
	if k := f.lookupClassVar(name); k != nil {
		return k
	}
	f.errorf(n, "uninitialized class variable %s (rb2go needs its first assignment in a class or module body)", name)
	return nil
}

func (f *fctx) lookupClassVar(name string) *Const {
	cls := f.owner
	if cls != nil && cls.metaOf != nil {
		cls = cls.metaOf
	}
	if cls == nil {
		return nil
	}
	for _, a := range cls.ancestors() {
		if k := a.cvars[name]; k != nil {
			return k
		}
	}
	// Ruby looks @@x up from the lexical class: a module's method copied into its includer or prepender reads the
	// module's (decision 241)
	if len(f.lex) > 0 {
		for _, a := range f.lex[len(f.lex)-1].ancestors() {
			if k := a.cvars[name]; k != nil {
				return k
			}
		}
	}
	return nil
}

// ---- ivars

func (f *fctx) ivar(n parser.Node, name string, assigned Type) *Ivar {
	if f.owner == nil || f.owner.universal || !f.owner.isStruct() && !f.owner.IsModule && !f.owner.hashBase {
		f.errorf(n, "instance variables are only supported in struct classes and modules")
	}
	if iv := f.owner.scratchIvars[name]; iv != nil {
		return iv
	}
	iv := f.c.findIvar(f.owner, name)
	f.nilableOnUntyped(iv, assigned)
	if iv == nil && f.owner.IsModule && f.lenient() { // gem code: an undeclared module ivar is untyped (decision 172)
		iv = f.c.declareIvar(f.owner, name, TAny{}, f.f, f.f.line(n.GetLocation().StartOffset))
	}
	declares := f.discover && assigned != nil && !isNil(assigned) && !isVoid(assigned) // decision 317: a module's own assignment declares it
	if iv == nil && f.owner.IsModule && !declares {
		f.errorf(n, "instance variable %s of module %s needs a type: `# @rbs %s: T` in the module body (decision 147)", name, f.owner.RubyName, name)
	}
	if iv == nil {
		if iv := f.nilIvar(name, assigned); iv != nil {
			return iv
		}
		if f.discover && assigned != nil && isNil(assigned) { // only ever nil: untyped after discovery (decision 169 for gems, 183 for user code)
			f.c.gemNilIvars = append(f.c.gemNilIvars, gemNilIvar{cls: f.owner, name: name, srcPos: srcPos{f.f, f.f.line(n.GetLocation().StartOffset)}})
		}
		if f.discover && assigned != nil && !isNil(assigned) && !isVoid(assigned) {
			assigned = subst(assigned, map[string]Type{"Self": TClass{C: f.owner}}) // `@me = self` in a generic body: the field holds the owner
			if f.c.ivarReadFirst[ivarKey{f.owner, name}] {
				assigned = optOf(assigned)
			}
			return f.c.declareIvar(f.owner, name, assigned, f.f, f.f.line(n.GetLocation().StartOffset))
		}
		f.errorf(n, "instance variable %s has no known type; assign it in initialize or add `# @rbs %s: T`", name, name)
	}
	return iv
}

// nilIvar notes a discovery-time `@x = nil` where @x has no type yet (decision 182): a subclass that typed it first
// gives it up to this class, nilable; otherwise a later subclass assignment declares it here.
func (f *fctx) nilIvar(name string, assigned Type) *Ivar {
	if !f.discover || assigned == nil || !isNil(assigned) || !f.owner.isStruct() {
		return nil
	}
	if f.c.nilIvars == nil {
		f.c.nilIvars = map[ivarKey]bool{}
	}
	f.c.nilIvars[ivarKey{f.owner, name}] = true
	return f.c.subclassIvar(f.owner, name)
}

func (f *fctx) ivarCode(iv *Ivar) string { return ivarPath(f.selfCode, iv) }

// readsUnassigned is a discovery read of an ivar no assignment has typed
// yet (`unless @x; @x = ...`): it reads nil, and the ivar is declared T? by
// the write that follows, since MRI's reads nil there (decision 148).
func (f *fctx) readsUnassigned(name string) bool {
	if !f.discover || f.owner == nil || f.c.findIvar(f.owner, name) != nil { // a module's too (decision 317)
		return false
	}
	if f.c.ivarReadFirst == nil {
		f.c.ivarReadFirst = map[ivarKey]bool{}
	}
	f.c.ivarReadFirst[ivarKey{f.owner, name}] = true
	return true
}

// narrowIvar reads iv as non-nil from here on in this scope (decision 148).
func (f *fctx) narrowIvar(iv *Ivar) {
	v := &local{name: "ivar:" + iv.Name, goName: f.ivarCode(iv), typ: iv.Type, declared: true}
	f.applyNarrow([]narrowInfo{{local: v, typ: stripOpt(iv.Type)}})
}

// ivarLocal is @x as a local genCond can narrow, like attrLocal's readers.
func (f *fctx) ivarLocal(n *parser.InstanceVariableReadNode) *local {
	if v := f.scope.lookup("ivar:" + n.Name); v != nil {
		return v
	}
	if f.readsUnassigned(n.Name) {
		return nil // genIvarExpr's nil
	}
	iv := f.ivar(n, n.Name, nil)
	return &local{name: "ivar:" + n.Name, goName: f.ivarCode(iv), typ: iv.Type, declared: true}
}

// afterCall forgets narrowed ivars once a call through self ran a method
// the program defines, which may have reassigned them; prelude methods and
// attribute readers leave self's ivars alone (decision 148).
func (f *fctx) afterCall(n *parser.CallNode) {
	if n.Receiver != nil && !isSelf(n.Receiver) || !f.hasNarrowedIvar() {
		return
	}
	var m *Method
	if f.owner != nil {
		if e := f.owner.lookup(n.Name); e != nil {
			m = e.M
		}
	}
	if m == nil {
		m = f.c.topDefs[n.Name]
	}
	if m != nil && (m.File != nil && m.File.isPrelude() || m.Kind == kindAttrReader) {
		return
	}
	f.unnarrowIvars()
}

func (f *fctx) hasNarrowedIvar() bool {
	for sc := f.scope; sc != nil; sc = sc.parent {
		for name, v := range sc.vars {
			if v.base != nil && strings.HasPrefix(name, "ivar:") {
				return true
			}
		}
	}
	return false
}

func (f *fctx) unnarrowIvars() {
	for sc := f.scope; sc != nil; sc = sc.parent {
		for name, v := range sc.vars {
			if v.base != nil && strings.HasPrefix(name, "ivar:") {
				delete(sc.vars, name)
			}
		}
	}
}

// isBlockParam reports whether n reads the method's own &block parameter.
func (f *fctx) isBlockParam(n parser.Node) bool {
	lv, ok := n.(*parser.LocalVariableReadNode)
	if !ok || f.m == nil || f.m.BlockParam == "" || lv.Name != f.m.BlockParam {
		return false
	}
	v := f.scope.lookup(lv.Name)
	return v == nil || v.goName == optBlockGo || v.base != nil && v.base.goName == optBlockGo
}

// blockParamValue reads the method's own &block as a Proc value, marking the
// block escaped so it takes `self` as a leading Go param (decision 164).
func (f *fctx) blockParamValue(n parser.Node) expr {
	b := f.blockSig
	if b == nil {
		f.errorf(n, "the block parameter &%s has no block signature", f.m.BlockParam)
	}
	return expr{code: "Ref(blk)", typ: TFunc{Params: b.Params, Ret: b.Ret, Proc: true, Rest: b.Rest, Self: f.blockSelf(n), SelfPending: b.SelfPending, Src: b.Src}}
}

// genLocalRead reads a local, or the method's own &block as a Proc value when
// it is stored (decision 164).
func (f *fctx) genLocalRead(n *parser.LocalVariableReadNode) expr {
	if f.isBlockParam(n) && f.m != nil && f.m.Block != nil && !f.m.Block.Optional && !f.m.Iterator {
		return f.blockParamValue(n)
	}
	v := f.readLocal(n)
	return expr{code: v.goName, typ: v.typ, view: v.view}
}

// attrLocal treats a receiver-less call of self's attribute reader as a
// local for narrowing: readers are pure, so `if resource.is_a?(Array)`
// may narrow `resource` for the branch, like Ruby programmers expect.
func (f *fctx) attrLocal(n parser.Node) *local {
	if iv, ok := n.(*parser.InstanceVariableReadNode); ok {
		return f.ivarLocal(iv)
	}
	call, ok := n.(*parser.CallNode)
	if !ok || call.Receiver != nil || call.Arguments != nil || call.Block != nil || f.owner == nil {
		return nil
	}
	if v := f.scope.lookup("attr:" + call.Name); v != nil {
		return v
	}
	e := f.owner.lookup(call.Name)
	if e == nil || e.M.Kind != kindAttrReader {
		return nil
	}
	x := f.genExpr(call, nil)
	return &local{name: "attr:" + call.Name, goName: x.code, typ: x.typ, declared: true}
}

// narrowString reads a String? local that was just assigned a String as String for the rest of its branch.
func (f *fctx) narrowString(name string) {
	v := f.visibleLocal(name)
	if v == nil || v.base != nil && typeEq(v.typ, f.cls("String")) { // assignLocal kept it narrowed
		return
	}
	t := v.typ
	if info := f.localInfo(name); info != nil && info.typ != nil && f.pass == 2 {
		t = info.typ // the Go variable's type is the join (rack's param is also a Symbol)
	}
	str := f.cls("String")
	switch {
	case f.c.goType(t) == "any":
		f.applyNarrow([]narrowInfo{{local: v, typ: str, code: v.goName + ".(" + f.c.goType(str) + ")"}})
	case isOpt(t) && isClass(t.(TOpt).Elem, "String"):
		f.applyNarrow([]narrowInfo{{local: v, typ: str}})
	}
}

// genIvarsEval is a gem's `child.instance_eval do @x = v; ... end` on an untyped receiver (an inherited hook seeding a
// subclass's class-level ivars): each write is rbIvarSet on the receiver, as instance_variable_set (decision 172).
func (f *fctx) genIvarsEval(b *parser.BlockNode, recv expr) (expr, bool) {
	st, ok := b.Body.(*parser.StatementsNode)
	if !f.lenient() || !isAny(recv.typ) || !ok || len(st.Body) == 0 {
		return expr{}, false
	}
	for _, x := range st.Body {
		if _, ok := x.(*parser.InstanceVariableWriteNode); !ok {
			return expr{}, false
		}
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, recv.code)
	last := expr{code: "nil", typ: TNil{}, lit: true}
	for _, x := range st.Body {
		w := x.(*parser.InstanceVariableWriteNode) //nolint:forcetypeassert // checked above
		v := f.genExpr(w.Value, nil)
		val := f.newTmp()
		f.emit("var %s any = %s", val, f.coerce(w.Value, v, TAny{}))
		f.emit("rbIvarSet(%s, Symbol(%q), %s)", tmp, w.Name, val)
		last = expr{code: val, typ: TAny{}}
	}
	return last, true
}

// annotatedIvarValue is `@x = v #: T` (or T.let): v as T, and T as the ivar's declared type.
func (f *fctx) annotatedIvarValue(n parser.Node, val expr, ann Type) (expr, Type) {
	ann = adoptBind(ann, val.typ)
	val = f.unionToAnn(n, val, ann)
	if isAny(val.typ) && !isAny(ann) { // `@args = T.let(untyped, Args)`: converted, not just relabeled
		val = expr{code: f.coerce(n, val, ann), typ: ann}
	}
	if tc, ok := ann.(TClass); ok && !typeEq(val.typ, ann) { // `@input = T.let(Array(input), T::Array[String])`: a converted copy
		if code, ok := f.lenientClass(n, val, tc); ok {
			val = expr{code: code, typ: ann}
		}
	}
	if o, ok := val.typ.(TOpt); ok && !isOpt(ann) && typeEq(o.Elem, ann) { // `@name = T.let(name, String)` of a String?
		if !f.lenient() {
			f.errorf(n, "possibly-nil %s where %s is expected; check it first (`if x`, `x ||= ...`, `return unless x`)", val.typ, ann)
		}
		val = expr{code: fmt.Sprintf("rbMust(%s, %q)", val.code, ann.String()), typ: ann} // nil raises, as sorbet-runtime's T.let does (decision 172)
	}
	if o, ok := ann.(TOpt); ok && !isOpt(val.typ) && !isNil(val.typ) && !isAny(val.typ) {
		val.typ = o.Elem // a T into a T? ivar: coerce wraps it
	} else {
		val.typ = ann
	}
	return val, ann // `@x = [] #: Array[T]` declares the ivar's type
}

// unionToAnn converts, rather than relabels, a union value written to an annotated ivar: a union is Go any.
func (f *fctx) unionToAnn(n parser.Node, val expr, ann Type) expr {
	if isUnion(val.typ) && !isUnion(ann) {
		return expr{code: f.coerce(n, val, ann), typ: ann}
	}
	return val
}

// byValueType is a type Go holds by value, with no nil of its own: a non-struct, immutable class (Symbol, String, Integer).
func byValueType(t Type) bool {
	cl, ok := t.(TClass)
	return ok && !cl.C.isStruct() && !cl.C.mutable() && !cl.C.IsModule
}

// nilableOnUntyped makes a gem ivar of a value type nilable when discovery assigns it an untyped value, which may be nil
// (kramdown's `@tree, @text_type = @stack.pop`): a Symbol field cannot hold it (decision 330).
func (f *fctx) nilableOnUntyped(iv *Ivar, assigned Type) {
	if iv != nil && f.discover && f.lenient() && isAny(assigned) && byValueType(iv.Type) {
		iv.Type = optOf(iv.Type)
	}
}
