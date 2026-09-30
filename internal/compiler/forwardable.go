package compiler

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// delegation is one Forwardable-generated method (decision 99): name calls
// method on accessor (`@ivar` or a method name).
type delegation struct {
	file     *File
	node     *parser.CallNode
	scope    []*Class
	accessor string
	method   string
	name     string
}

// collectDelegation records `def_delegators :@x, :a, :b`,
// `def_delegator :@x, :a, :alias` and `delegate [:a, :b] => :@x`.
func (c *Compiler) collectDelegation(f *File, cls *Class, n *parser.CallNode, args []parser.Node, scope []*Class) {
	sym := func(a parser.Node) string {
		s, ok := a.(*parser.SymbolNode)
		if !ok {
			c.errorf(f, n, "%s takes literal Symbols", n.Name)
		}
		return s.Unescaped.Value
	}
	add := func(accessor, method, name string) {
		cls.delegations = append(cls.delegations, delegation{file: f, node: n, scope: scope, accessor: accessor, method: method, name: name})
	}
	switch n.Name {
	case "def_delegators":
		if len(args) < 2 {
			c.errorf(f, n, "def_delegators needs an accessor and method names")
		}
		for _, a := range args[1:] {
			add(sym(args[0]), sym(a), sym(a))
		}
	case "def_delegator":
		if len(args) < 2 || len(args) > 3 {
			c.errorf(f, n, "def_delegator takes an accessor, a method and an optional alias")
		}
		name := sym(args[1])
		if len(args) == 3 {
			name = sym(args[2])
		}
		add(sym(args[0]), sym(args[1]), name)
	default: // delegate [:a, :b] => :@x, :c => :@y
		h, ok := args[0].(*parser.KeywordHashNode)
		if len(args) != 1 || !ok {
			c.errorf(f, n, "%s takes `[:methods] => :accessor` pairs", n.Name)
		}
		for _, el := range h.Elements {
			as, ok := el.(*parser.AssocNode)
			if !ok {
				c.errorf(f, n, "%s takes `[:methods] => :accessor` pairs", n.Name)
			}
			acc := sym(as.Value)
			if arr, ok := as.Key.(*parser.ArrayNode); ok {
				for _, m := range arr.Elements {
					add(acc, sym(m), sym(m))
				}
				continue
			}
			add(acc, sym(as.Key), sym(as.Key))
		}
	}
}

// expandDelegations turns each delegation into an ordinary def, written
// out as Ruby with the target method's signature: `def size = @items.size`
// under `#: () -> Integer`. It runs in link once supers and includes are
// known and before signatures resolve, so the accessor's type must be
// declared: `# @rbs @items: T`, a `#:` on its assignment in initialize,
// or an annotated reader.
func (c *Compiler) expandDelegations(ctx context.Context) {
	added := false
	for _, cls := range c.classList {
		for _, d := range cls.delegations {
			c.expandDelegation(ctx, cls, d)
			added = true
		}
	}
	if added { // lookups above cached method sets without the new defs
		for _, cls := range c.classList {
			cls.msetCache = nil
		}
	}
}

func (c *Compiler) expandDelegation(ctx context.Context, cls *Class, d delegation) {
	f, n := d.file, d.node
	recvT := c.delegateTarget(cls, d)
	rc, ok := recvT.(TClass)
	if !ok {
		c.errorf(f, n, "%s: cannot delegate to %s of type %s", n.Name, d.accessor, recvT)
	}
	e := rc.C.lookup(d.method)
	if e == nil {
		c.errorf(f, n, "%s: undefined method %s for %s", n.Name, d.method, recvT)
	}
	c.resolveMethod(e.M)
	m := e.M
	if m.inferRet && m.Ret == nil {
		c.errorf(f, n, "%s: %s#%s needs a return type annotation to be delegated to", n.Name, recvT, d.method)
	}
	env := composeEnv(e.Env, nil)
	for i, p := range rc.C.TypeParams {
		if i < len(rc.Args) {
			env[p] = rc.Args[i]
		}
	}
	env["Self"] = recvT
	required := 0
	for _, p := range m.Params {
		if !p.Rest && p.Default == nil {
			required++
		}
	}
	// One def per arity, as decision 12's overloads: omitted optional
	// arguments stay omitted, so the target runs its own defaults.
	c.delegateDef(ctx, cls, d, m, env, required, d.name, true)
	for i, p := range m.Params {
		if p.Default != nil {
			c.delegateDef(ctx, cls, d, m, env, i+1, fmt.Sprintf("__%s_%d", d.name, i+1), m.Block != nil)
		}
	}
}

// delegateDef writes one delegated method taking the target's first
// count positional parameters (and its rest parameter, if any). With
// full, it carries the target's whole signature; otherwise per-parameter
// annotations, and its return is inferred from the forwarded call.
func (c *Compiler) delegateDef(ctx context.Context, cls *Class, d delegation, m *Method, env map[string]Type, count int, name string, full bool) {
	f, n := d.file, d.node
	var ps, params, args []string
	var anns []string
	i := 0
	for _, p := range m.Params {
		if !p.Rest && i >= count {
			continue
		}
		a := fmt.Sprintf("a%d", len(params))
		t := subst(p.Type, env)
		if p.Rest {
			ps, params, args = append(ps, "*"+t.String()), append(params, "*"+a), append(args, "*"+a)
			anns = append(anns, fmt.Sprintf("# @rbs *%s: %s", a, t))
			continue
		}
		i++
		ps, params, args = append(ps, t.String()), append(params, a), append(args, a)
		anns = append(anns, fmt.Sprintf("# @rbs %s: %s", a, t))
	}
	sig := "(" + strings.Join(ps, ", ") + ")"
	if len(m.TypeParams) > 0 {
		sig = "[" + strings.Join(m.TypeParams, ", ") + "] " + sig
	}
	block := ""
	if m.Block != nil {
		bs := make([]string, len(m.Block.Params))
		ts := make([]string, len(m.Block.Params))
		for i, t := range m.Block.Params {
			bs[i] = fmt.Sprintf("b%d", i)
			ts[i] = subst(t, env).String()
		}
		sig += fmt.Sprintf(" { (%s) -> %s }", strings.Join(ts, ", "), subst(m.Block.Ret, env))
		block = fmt.Sprintf(" { |%s| yield %s }", strings.Join(bs, ", "), strings.Join(bs, ", "))
	}
	sig += " -> " + subst(m.Ret, env).String()
	call := d.accessor
	if d.method == "[]" {
		call += "[" + strings.Join(args, ", ") + "]"
	} else {
		call += "." + d.method + "(" + strings.Join(args, ", ") + ")"
	}
	head := "#: " + sig
	if !full {
		head = strings.Join(anns, "\n")
	}
	// padded so the def's line (errors, //line) is the def_delegators call's
	line := f.line(n.Location.StartOffset)
	src := strings.Repeat("\n", max(line-1-strings.Count(head, "\n")-1, 0)) + head + "\ndef " + name + "(" + strings.Join(params, ", ") + ")\n  " + call + block + "\nend\n"
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), f.prelude)
	if err != nil {
		c.errorf(f, n, "%s %s: %v", n.Name, d.name, err)
	}
	c.addDef(sf, cls, sf.Root.Statements.Body[0].(*parser.DefNode), false, d.scope)
}

// delegateTarget is the declared type of a delegation's accessor.
func (c *Compiler) delegateTarget(cls *Class, d delegation) Type {
	f, n := d.file, d.node
	sc := typeScope{class: cls, lex: d.scope, file: f, line: f.line(n.Location.StartOffset)}
	if !strings.HasPrefix(d.accessor, "@") {
		e := cls.lookup(d.accessor)
		if e == nil {
			c.errorf(f, n, "%s: undefined method %s", n.Name, d.accessor)
		}
		c.resolveMethod(e.M)
		if e.M.Ret == nil {
			c.errorf(f, n, "%s: %s needs a return type annotation to be delegated through", n.Name, d.accessor)
		}
		return e.M.Ret
	}
	if iv := c.findIvar(cls, d.accessor); iv != nil && iv.Type != nil {
		return iv.Type
	}
	for k := cls; k != nil; k = k.Super {
		init := k.Methods["initialize"]
		if init == nil || init.Node == nil {
			continue
		}
		var ann string
		anyNode(init.Node.Body, func(x parser.Node) bool {
			if w, ok := x.(*parser.InstanceVariableWriteNode); ok && w.Name == d.accessor && ann == "" {
				ann = init.File.trailingAnnotation(w)
			}
			return false
		})
		if ann != "" {
			t, err := rbs.ParseType(ann)
			if err != nil {
				c.errorf(f, n, "%v", err)
			}
			return c.resolveType(t, sc)
		}
	}
	c.errorf(f, n, "%s: the type of %s must be declared to delegate to it (`# @rbs %s: T`, or `%s = ... #: T` in initialize)", n.Name, d.accessor, d.accessor, d.accessor)
	return nil
}
