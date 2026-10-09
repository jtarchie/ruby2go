package compiler

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/jtarchie/ruby2go/internal/rbs"
)

// loadSigs parses .rbs signature files beside each user source (a `sig/`
// directory next to the file) and the vendored gem signatures (decision 160:
// `sig/gems/<gem>/`), keyed by qualified class name. Inline `#:` annotations
// win; a sig is consulted only where one is missing.
func (c *Compiler) loadSigs(sources []Source) {
	seen := map[string]bool{}
	for _, src := range sources {
		p := src.Path
		if p == "" {
			p = src.Name
		}
		dir := filepath.Join(filepath.Dir(p), "sig")
		if seen[dir] {
			continue
		}
		seen[dir] = true
		c.loadSigDir(dir)
	}
	c.loadGemSigs()
}

// loadGemSigs parses the vendored gem signatures embedded in the compiler's
// gemSigs FS (decision 160). They are keyed by class name like a user's, so a
// program that carries a class of the same qualified name gets its types.
func (c *Compiler) loadGemSigs() {
	if c.gemSigs == nil {
		return
	}
	paths, _ := fs.Glob(c.gemSigs, "sig/gems/*/*.rbs") // the pattern is constant
	for _, p := range paths {
		data, err := fs.ReadFile(c.gemSigs, p)
		if err != nil {
			c.errorf(nil, nil, "%s: %v", p, err)
		}
		f, perr := rbs.ParseFile(string(data))
		if perr != nil {
			c.errorf(nil, nil, "%s: %v", p, perr)
		}
		for _, decl := range f.Decls {
			c.sigs[decl.Name] = decl
		}
	}
}

func (c *Compiler) loadSigDir(dir string) {
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".rbs") {
			return nil
		}
		data, rerr := os.ReadFile(path) //nolint:gosec // the program's own sig directory
		if rerr != nil {
			return fmt.Errorf("read %s: %w", path, rerr)
		}
		f, perr := rbs.ParseFile(string(data))
		if perr != nil {
			c.errorf(nil, nil, "%s: %v", path, perr)
		}
		for _, decl := range f.Decls {
			c.sigs[decl.Name] = decl
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		c.errorf(nil, nil, "%v", err)
	}
}

// sigMethod is the sig file's method `name` on cls (a `def self.` when cls is
// a metaclass), or nil.
func (c *Compiler) sigMethod(cls *Class, name string) *rbs.MethodDecl {
	if cls == nil {
		return nil
	}
	d := c.sigs[cls.RubyName]
	if d == nil {
		return nil
	}
	wantSelf := cls.metaOf != nil
	for _, m := range d.Methods {
		if m.Name == name && (m.Self == wantSelf || m.Both) {
			return m
		}
	}
	return nil
}

// sigAttr is the sig file's attribute `name` on cls, of any kind (an `attr_* self.` one when cls is a metaclass).
func (c *Compiler) sigAttr(cls *Class, name string) *rbs.AttrDecl {
	if cls == nil {
		return nil
	}
	d := c.sigs[cls.RubyName]
	if d == nil {
		return nil
	}
	wantSelf := cls.metaOf != nil
	for _, a := range d.Attrs {
		if a.Name == name && a.Self == wantSelf {
			return a
		}
	}
	return nil
}

// sigTypeParams is the sig file's type parameters for cls, or nil.
func (c *Compiler) sigTypeParams(cls *Class) []string {
	if cls == nil {
		return nil
	}
	if d := c.sigs[cls.RubyName]; d != nil {
		return d.TypeParams
	}
	return nil
}

// sigConst is the sig file's type for constant k (`NAME: T` in its class), or nil.
func (c *Compiler) sigConst(k *Const) rbs.Type {
	if k.File == nil || k.File.prelude {
		return nil
	}
	owner, name := "", k.RubyName
	if i := strings.LastIndex(name, "::"); i >= 0 {
		owner, name = name[:i], name[i+2:]
	}
	if d := c.sigs[owner]; d != nil {
		for _, cd := range d.Consts {
			if cd.Name == name {
				return cd.Type
			}
		}
	}
	return nil
}

// addSigIvars declares each user class's `.rbs` instance variables (`@x: T`, `self.@x: T`) that no inline
// `# @rbs` annotation already declares (decision 160).
func (c *Compiler) addSigIvars() {
	for _, cls := range c.classList {
		d := c.sigs[cls.RubyName]
		if d == nil || cls.metaOf != nil || cls.File == nil || cls.File.prelude {
			continue
		}
		for _, iv := range d.Ivars {
			if !slices.ContainsFunc(cls.ivarDecls, func(x ivarDecl) bool { return x.name == iv.Name }) {
				cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: iv.Name, rbs: iv.Type, line: cls.Line})
			}
		}
	}
}

// mergeArms is the one signature a `.rbs` method's overloads compile under
// (decision 12): each parameter is the union of the arms' types at its
// position, optional where an arm omits it, and the return the union of
// theirs. Nil when the arms cannot share a Go signature.
func mergeArms(sd *rbs.MethodDecl) *rbs.MethodType {
	if sd == nil || len(sd.Overloads) < 2 {
		return nil
	}
	first := sd.Overloads[0]
	out := &rbs.MethodType{TypeParams: first.TypeParams, Block: first.Block}
	var rets []rbs.Type
	for _, arm := range sd.Overloads {
		// ponytail: positional-only arms sharing one block; keyword/rest arms and differing blocks fall back to inference
		if blockText(arm.Block) != blockText(first.Block) || !slices.Equal(arm.TypeParams, first.TypeParams) {
			return nil
		}
		for i, p := range arm.Params {
			if p.Rest || p.Keyword || p.KwRest {
				return nil
			}
			if i == len(out.Params) {
				out.Params = append(out.Params, rbs.Param{Name: p.Name, Optional: arm != first})
			}
			out.Params[i].Type = unionAdd(out.Params[i].Type, p.Type)
			out.Params[i].Optional = out.Params[i].Optional || p.Optional
		}
		for i := len(arm.Params); i < len(out.Params); i++ {
			out.Params[i].Optional = true
		}
		rets = append(rets, arm.Return)
	}
	out.Return = rets[0]
	for _, r := range rets[1:] {
		out.Return = unionAdd(out.Return, r)
	}
	return out
}

func blockText(b *rbs.Block) string {
	if b == nil {
		return ""
	}
	return (&rbs.MethodType{Block: b, Return: rbs.Void{}}).String()
}

// unionAdd is `a | b`, dropping b when a already spells it.
func unionAdd(a, b rbs.Type) rbs.Type {
	if a == nil {
		return b
	}
	elems := []rbs.Type{a}
	if u, ok := a.(rbs.Union); ok {
		elems = u.Elems
	}
	for _, e := range elems {
		if e.String() == b.String() {
			return a
		}
	}
	return rbs.Union{Elems: append(slices.Clone(elems), b)}
}

// armRet is the return type of the first of m's `.rbs` overloads the
// call's arguments fit (decision 12): by count, then each argument's class,
// then the literal value of a literal or constant argument against a
// literal type.
func (f *fctx) armRet(n parser.Node, m *Method, args []parser.Node) Type {
	sc := typeScope{class: m.Owner, lex: m.Scope, file: m.File, line: m.Line}
arms:
	for _, arm := range m.arms {
		req := 0
		for _, p := range arm.Params {
			if !p.Optional {
				req++
			}
		}
		if len(args) < req || len(args) > len(arm.Params) {
			continue
		}
		sc.methodTPs = arm.TypeParams
		for i, a := range args {
			if !f.argFits(a, arm.Params[i].Type, sc) {
				continue arms
			}
		}
		return f.c.resolveType(arm.Return, sc)
	}
	arms := make([]string, len(m.arms))
	for i, a := range m.arms {
		arms[i] = a.String()
	}
	f.errorf(n, "no signature of %s matches this call: %s", m.Name, strings.Join(arms, " | "))
	return nil
}

// argFits reports whether argument a may be passed as an arm's parameter of type t.
func (f *fctx) argFits(a parser.Node, t rbs.Type, sc typeScope) bool {
	switch t := t.(type) {
	case rbs.Literal:
		v, ok := f.c.literalOf(f.f, a, f.lex)
		return ok && v == t
	case rbs.Union:
		return slices.ContainsFunc(t.Elems, func(e rbs.Type) bool { return f.argFits(a, e, sc) })
	case rbs.Bool, rbs.Bot, rbs.Name, rbs.Nil, rbs.Optional, rbs.Proc, rbs.Record, rbs.Self, rbs.Singleton, rbs.Tuple, rbs.Untyped, rbs.Void: // by the argument's class
	}
	var at expr
	f.probe(func() { at = f.genExpr(a, nil) })
	return fits(at.typ, f.c.resolveType(t, sc))
}

// literalOf is the literal type node n spells, following constants to their value (`Rack::PATH_INFO`).
func (c *Compiler) literalOf(file *File, n parser.Node, lex []*Class) (rbs.Literal, bool) {
	switch n := n.(type) {
	case *parser.StringNode:
		return rbs.Literal{Kind: "String", Value: n.Unescaped.Value}, true
	case *parser.SymbolNode:
		return rbs.Literal{Kind: "Symbol", Value: n.Unescaped.Value}, true
	case *parser.IntegerNode:
		return rbs.Literal{Kind: "Integer", Value: file.text(n.Location)}, true
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		if _, k := c.lookupConst(file, n, lex); k != nil && k.Value != nil {
			return c.literalOf(k.File, k.Value, k.Scope)
		}
	}
	return rbs.Literal{}, false
}

// narrowToArm types a call by its overload arm's return: the merged
// result, a union or optional, narrows to it, checked at run time.
func (f *fctx) narrowToArm(n parser.Node, out expr, ret Type) expr {
	switch {
	case typeEq(out.typ, ret) || isAny(ret):
		return out
	case isOpt(out.typ) && !isOpt(ret) && !isUnion(out.typ.(TOpt).Elem):
		out = expr{code: fmt.Sprintf("rbMust(%s, %q)", out.code, ret.String()), typ: out.typ.(TOpt).Elem}
		return f.narrowToArm(n, out, ret)
	case isUnion(out.typ) || isAny(out.typ):
		switch r := ret.(type) {
		case TOpt:
			return expr{code: fmt.Sprintf("OptOf[%s](%s, %q)", f.c.goType(r.Elem), out.code, r.Elem.String()), typ: ret}
		default:
			return expr{code: out.code + ".(" + f.c.goType(ret) + ")", typ: ret, assert: true}
		}
	}
	return expr{code: f.coerce(n, out, ret), typ: ret}
}
