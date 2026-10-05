package compiler

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Parameter types from use (decision 146). A def parameter with no
// annotation takes the join of what the program passes it. A compile that
// meets one first aborts (errNeedInfer); compile then runs rounds in which
// such parameters are untyped and every user body is dry-run to record the
// arguments' types, each round typing what the last one learned, until
// nothing changes. The final compile uses the types; a parameter no call
// types keeps the missing-annotation error, and a call whose argument does
// not join the earlier ones (in source order) fails at that call, as a
// mismatch does against an annotation.

var (
	errNeedInfer  = errors.New("parameter types to infer")
	errInferRound = errors.New("inference round done")
)

// needInfer is the panic that aborts a compile meeting an untyped parameter before inference ran.
type needInfer struct{}

const maxInferRounds = 8

// inference is what the rounds learned: each parameter key's type, from
// the last round's compile (rehome moves it into the next one's classes).
type inference struct {
	types   map[string]Type
	from    map[string]string // the first call each type came from, file:line, for a mismatch's message
	changed bool
}

// pendingParam is a parameter whose type comes from use: its key across compiles, and its name for messages.
type pendingParam struct {
	key, name string
}

// paramUse is one argument passed to a pending parameter.
type paramUse struct {
	typ  Type
	file *File
	off  int
}

// pendingKey names a def's parameter stably across compiles.
func pendingKey(m *Method, name string) string {
	path := m.File.path
	if path == "" {
		path = m.File.Name
	}
	return path + ":" + strconv.Itoa(m.Node.Location.StartOffset) + ":" + name
}

// resolvePending types a parameter that has no annotation (resolveMethod).
func (c *Compiler) resolvePending(m *Method, prm *Param, pp pendingParam) {
	prm.Pending = pp.key
	if c.infer != nil {
		if t, ok := c.infer.types[pp.key]; ok {
			prm.Type = c.rehome(t)
			return
		}
	}
	switch {
	case c.round:
		prm.Type = TAny{} // learned from this round's calls
	case !c.inferDone:
		panic(needInfer{})
	default:
		c.errorf(m.File, m.Node, "method %s has no type for parameter %s (`# @rbs %s: T` or `#: (...) -> T`), and no call in the program gives it one (decision 146)", m.Name, strings.TrimLeft(pp.name, "*"), pp.name)
	}
}

// yieldShape is the block an unannotated def yields to: how many values
// each yield passes (all must agree), and whether block_given? makes it
// optional. ok is false for a def that does not yield, or names its block.
func (c *Compiler) yieldShape(m *Method) (n int, optional, ok bool) {
	if m.Node == nil || m.Node.Body == nil || m.BlockParam != "" {
		return 0, false, false
	}
	n = -1
	agree := true
	var walk func(parser.Node)
	walk = func(x parser.Node) {
		switch x := x.(type) {
		case nil, *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
			return
		case *parser.YieldNode:
			k := 0
			if x.Arguments != nil {
				k = len(x.Arguments.Arguments)
			}
			if n >= 0 && k != n && agree {
				agree = false
				c.errorf(m.File, x, "method %s yields %d values here and %d before; annotate its block (`#: (...) { (T, ...) -> void } -> R`)", m.Name, k, n)
			}
			n = k
		case *parser.CallNode:
			if x.Receiver == nil && x.Name == "block_given?" {
				optional = true
			}
		}
		for _, ch := range x.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(m.Node.Body)
	return n, optional, n >= 0 && agree
}

// includeFromEach types `include Enumerable` written without type args
// from the includer's own `each`: what it yields is the element type, as
// MRI's Enumerable has whatever each yields.
func (c *Compiler) includeFromEach() {
	for _, cls := range c.classList {
		for i := range cls.Includes {
			inc := &cls.Includes[i]
			if !inc.fromEach {
				continue
			}
			each := cls.Methods["each"]
			c.resolveMethod(each)
			if each.Block == nil || len(each.Block.Params) != len(inc.Mod.TypeParams) {
				c.errorf(inc.file, nil, "%s:%d: include %s needs %d type args (`include %s #[...]`): %s#each does not yield %d values to infer them from", inc.file.Name, inc.line, inc.Mod.RubyName, len(inc.Mod.TypeParams), inc.Mod.RubyName, cls.RubyName, len(inc.Mod.TypeParams))
			}
			inc.Args = slices.Clone(each.Block.Params)
			var reset func(*Class) // method sets built meanwhile hold the placeholder args
			reset = func(k *Class) {
				k.msetCache, k.msetIndex = nil, nil
				for _, s := range k.Subclasses {
					reset(s)
				}
			}
			reset(cls)
		}
	}
}

// typesInclude reports an `each` whose class's include takes its type args
// from it: it has its own signature, not the module's.
func typesInclude(m *Method) bool {
	if m.Name != "each" || m.Owner == nil {
		return false
	}
	for _, inc := range m.Owner.Includes {
		if inc.fromEach {
			return true
		}
	}
	return false
}

// lambdaParams types a lambda's parameters from its calls, when nothing
// gives it an expected type: the last round's types, untyped during a
// round, else the error an annotation would have prevented.
func (f *fctx) lambdaParams(n parser.Node, count int) ([]Type, string) {
	path := f.f.path
	if path == "" {
		path = f.f.Name
	}
	src := path + ":" + strconv.Itoa(n.GetLocation().StartOffset) + ":#"
	ps := make([]Type, count)
	for i := range ps {
		if f.c.infer != nil {
			if t, ok := f.c.infer.types[src+strconv.Itoa(i)]; ok {
				ps[i] = f.c.rehome(t)
				continue
			}
		}
		switch {
		case f.c.round:
			ps[i] = TAny{}
		case !f.c.inferDone:
			panic(needInfer{})
		default:
			f.errorf(n, "a lambda with parameters needs a type annotation (`#: ^(T) -> R`), and no call in the program gives it one (decision 146)")
		}
	}
	return ps, src
}

// sameParamShape reports whether def m's parameter list fits parent's
// signature: positional count, rest and keyword names. An unannotated
// override inherits the signature only then.
func (c *Compiler) sameParamShape(m, parent *Method) bool {
	if m.Node == nil || m.Kind != kindDef {
		return true
	}
	names, _, rest := c.defParams(m, m.Node.Parameters)
	npos, hasRest := 0, false
	var kws []string
	for _, p := range parent.Params {
		switch {
		case p.Keyword && !p.KwRest:
			kws = append(kws, p.Name)
		case p.Keyword:
		case p.Rest:
			hasRest = true
		default:
			npos++
		}
	}
	var own []string
	for _, k := range c.defKeywords(m, m.Node.Parameters) {
		if !k.rest {
			own = append(own, k.name)
		}
	}
	slices.Sort(kws)
	slices.Sort(own)
	return npos == len(names) && hasRest == (rest != "") && slices.Equal(kws, own)
}

// rehome rebuilds a type from an earlier compile with this one's classes, by name.
func (c *Compiler) rehome(t Type) Type {
	switch t := t.(type) {
	case TClass:
		var cls *Class
		if t.C.metaOf != nil {
			if o := c.classes[t.C.metaOf.RubyName]; o != nil {
				cls = o.meta
			}
		} else {
			cls = c.classes[t.C.RubyName]
		}
		if cls == nil {
			return TAny{}
		}
		args := make([]Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = c.rehome(a)
		}
		return TClass{C: cls, Args: args}
	case TOpt:
		return TOpt{Elem: c.rehome(t.Elem)}
	case TTuple:
		els := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			els[i] = c.rehome(e)
		}
		return TTuple{Elems: els}
	case TFunc:
		ps := make([]Type, len(t.Params))
		for i, p := range t.Params {
			ps[i] = c.rehome(p)
		}
		t.Params, t.Ret = ps, c.rehome(t.Ret)
		return t
	case TAny, TNil, TVar, TVoid: // no classes inside
	}
	return t
}

// noteYield records what a yield passes to a block parameter typed from the yields.
func (f *fctx) noteYield(i int, n parser.Node, t Type) {
	if i < len(f.blockSig.Pending) && f.blockSig.Pending[i] != "" {
		f.noteUse(Param{Pending: f.blockSig.Pending[i]}, n, t)
	}
}

// noteUse records an argument for a pending parameter, during a round.
func (f *fctx) noteUse(p Param, n parser.Node, t Type) {
	if p.Pending == "" || f.c.uses == nil || n == nil {
		return
	}
	f.c.uses[p.Pending] = append(f.c.uses[p.Pending], paramUse{typ: t, file: f.f, off: n.GetLocation().StartOffset})
}

// pendingWant is the expected type for an argument: none for a pending parameter in a round, so the argument types itself.
func (f *fctx) pendingWant(p Param, t Type) Type {
	if p.Pending != "" && f.c.round && isAny(p.Type) {
		return nil
	}
	return t
}

// collectUses dry-runs every user body for the arguments it passes.
func (c *Compiler) collectUses() {
	c.uses = map[string][]paramUse{}
	c.loadCode = map[*File]string{}
	for _, m := range c.userMethods() {
		c.debugInfer(catchCompileError(func() {
			if m.inferRet {
				m.Ret = nil
				c.inferRet(m)
			}
			if m.Owner == nil {
				c.emitTopDef(m)
			} else {
				c.emitMethod(m)
			}
		}))
		c.out.Reset()
		for _, p := range m.Params {
			if p.Pending == "" || p.Default == nil {
				continue
			}
			_ = catchCompileError(func() { // a default passed by no call still types its parameter
				f := c.newFctx(m.File, m.Owner, m)
				f.probe(func() { f.noteUse(p, p.Default, f.genExpr(p.Default, nil).typ) })
			})
		}
	}
	_, all := c.mainBodies()
	for _, b := range all {
		c.debugInfer(catchCompileError(func() {
			f := c.newFctx(b.f, nil, nil)
			f.retVar = ""
			f.genBody(&parser.StatementsNode{Body: b.stmts}, nil, tail{}, nil)
		}))
	}
}

// debugInfer prints what a round's dry run could not compile, under RB2GO_INFER_DEBUG.
func (c *Compiler) debugInfer(err *compileError) {
	if err != nil && os.Getenv("RB2GO_INFER_DEBUG") != "" {
		fmt.Fprintln(os.Stderr, "rb2go: infer round:", err.msg)
	}
}

func (c *Compiler) userMethods() []*Method {
	var out []*Method
	for _, m := range c.topDefList {
		if !m.File.prelude && m.Kind == kindDef {
			out = append(out, m)
		}
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if !m.File.prelude && m.Kind == kindDef {
				out = append(out, m)
			}
		}
	}
	return out
}

// updateInference joins each pending parameter's uses, in source order: a
// use that does not join the ones before it is left out, so the final
// compile reports it at its call.
func (c *Compiler) updateInference(inf *inference) {
	order := map[*File]int{}
	for i, f := range c.files {
		order[f] = i
	}
	next, from := map[string]Type{}, map[string]string{}
	for _, key := range slices.Sorted(maps.Keys(c.uses)) {
		seen := map[[2]int]int{}
		var uses []paramUse
		for _, u := range c.uses[key] { // a body generates more than once: the last pass's type
			k := [2]int{order[u.file], u.off}
			if i, ok := seen[k]; ok {
				uses[i] = u
				continue
			}
			seen[k] = len(uses)
			uses = append(uses, u)
		}
		slices.SortFunc(uses, func(a, b paramUse) int {
			return cmp.Or(cmp.Compare(order[a.file], order[b.file]), cmp.Compare(a.off, b.off))
		})
		var t Type
		for _, u := range uses {
			if _, void := u.typ.(TVoid); u.typ == nil || void || holdsAny(u.typ) || mentionsVar(u.typ) {
				continue // untyped this round (another pending parameter's), or no value
			}
			if t == nil {
				t = u.typ
				from[key] = fmt.Sprintf("%s:%d", u.file.Name, u.file.line(u.off))
				continue
			}
			if j, ok := join(t, u.typ); ok && !isAny(j) {
				t = j
			}
		}
		if t != nil {
			next[key] = t
		}
	}
	inf.changed = len(next) != len(inf.types)
	for k, t := range next {
		if old, ok := inf.types[k]; !ok || fmt.Sprint(old) != fmt.Sprint(t) {
			inf.changed = true
		}
	}
	inf.types, inf.from = next, from
	if os.Getenv("RB2GO_INFER_DEBUG") != "" {
		for _, k := range slices.Sorted(maps.Keys(next)) {
			fmt.Fprintf(os.Stderr, "rb2go: inferred %s: %s\n", k, next[k])
		}
	}
}

// coercePending is coerceArg's check for a parameter typed from use: a
// mismatch names the call its type came from.
func (f *fctx) coercePending(p Param, code func() string) string {
	if p.Pending == "" || f.c.infer == nil || f.c.round {
		return code()
	}
	var out string
	err := catchCompileError(func() { out = code() })
	if err != nil {
		if from := f.c.infer.from[p.Pending]; from != "" {
			err.msg += fmt.Sprintf("; parameter %s takes its type from the call at %s (decision 146): annotate it to take both", strings.TrimLeft(p.Name, "*"), from)
		}
		panic(*err)
	}
	return out
}
