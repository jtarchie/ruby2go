package compiler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/boot"
)

// No run-time eval: what boot's string evals defined is collected as source where it ran, and the call is a no-op (decision 204).

// collectBootEvals collects each recorded string eval whose call site is compiled, into the module it ran on.
func (c *Compiler) collectBootEvals(ctx context.Context, evals []boot.EvalInfo) {
	for _, e := range evals {
		c.keepGoing(func() { c.collectBootEval(ctx, e) })
	}
}

// splitLoc splits Ruby's "path:line" at the last colon: the path may hold one.
func splitLoc(loc string) (string, int) {
	i := strings.LastIndexByte(loc, ':')
	if i < 0 {
		return loc, 0
	}
	n, err := strconv.Atoi(loc[i+1:])
	if err != nil {
		return loc, 0
	}
	return loc[:i], n
}

// callerFile is the compiled file and line of a boot "path:line", or nil.
func (c *Compiler) callerFile(loc string) (*File, int) {
	path, ln := splitLoc(loc)
	if ln == 0 {
		return nil, 0
	}
	return c.parsed[c.real(path)], ln
}

func (c *Compiler) collectBootEval(ctx context.Context, e boot.EvalInfo) {
	f, ln := c.callerFile(e.Caller)
	if f == nil {
		return // evaluated by code outside the closed world (a replaced gem's own)
	}
	if e.Kind == "class_eval_block" {
		c.collectBlockEval(ctx, f, ln, e)
		return
	}
	c.evalSites[fmt.Sprintf("%s:%d", f.path, ln)] = true
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(e.Source), originSource)
	if err != nil {
		c.errorAt(srcPos{f, ln}, "eval: %v", err)
	}
	sf.path, sf.pinLine = f.path, ln
	target := c.classes[e.Self]
	switch {
	case e.Kind == "class_eval" && target != nil:
		c.collectBody(ctx, sf, target, sf.Root.Statements, c.lexicalOf(target), true)
	case e.Kind == "instance_eval" && target != nil:
		for _, st := range sf.Root.Statements.Body {
			switch n := st.(type) {
			case *parser.DefNode: // an instance_eval'd def is the module's singleton method
				target.singletonDefs = append(target.singletonDefs, singletonDef{node: n, scope: c.lexicalOf(target), file: sf})
			case *parser.ClassNode:
				c.collectClass(ctx, sf, n, nil)
			case *parser.ModuleNode:
				c.collectModule(ctx, sf, n, nil)
			default:
				c.errorf(sf, st, "instance_eval of %s: only defs and classes are compiled from boot (decision 204)", e.Self)
			}
		}
	case e.Kind == "eval" && e.Self == "":
		c.collect(ctx, sf)
	default:
		c.errorAt(srcPos{f, ln}, "a %s on %q: rb2go compiles a string eval only on a named module, from boot (decision 204)", e.Kind, e.Self)
	}
}

// lexicalOf is the lexical scope code written inside cls's body sees: each enclosing namespace, then cls.
func (c *Compiler) lexicalOf(cls *Class) []*Class {
	var out []*Class
	parts := strings.Split(cls.RubyName, "::")
	for i := range parts {
		if k := c.classes[strings.Join(parts[:i+1], "::")]; k != nil {
			out = append(out, k)
		}
	}
	return out
}

// isEvalSite reports a string eval boot ran here: its source is compiled, so the call does nothing (decision 204).
func (f *fctx) isEvalSite(n *parser.CallNode) bool {
	switch n.Name {
	case "eval", "class_eval", "module_eval", "instance_eval":
	default:
		return false
	}
	if _, ok := firstArg(callArgs(n)).(*parser.StringNode); !ok {
		if _, ok := firstArg(callArgs(n)).(*parser.InterpolatedStringNode); !ok {
			return false
		}
	}
	loc := n.Location
	if n.MessageLoc != nil {
		loc = *n.MessageLoc
	}
	return f.c.evalSites[fmt.Sprintf("%s:%d", f.f.path, f.f.line(loc.StartOffset))]
}

// collectBlockEval collects `klass.class_eval do ... end`'s block, at line ln of f, as a body of the module boot ran it
// on; the call is a no-op (decision 204). A block evaluated on several modules is each one's body.
func (c *Compiler) collectBlockEval(ctx context.Context, f *File, ln int, e boot.EvalInfo) {
	target := c.classes[e.Self]
	var call *parser.CallNode
	anyNode(f.Root, func(n parser.Node) bool {
		if x, ok := n.(*parser.CallNode); ok && (x.Name == "class_eval" || x.Name == "module_eval") {
			if b, ok := x.Block.(*parser.BlockNode); ok && f.line(b.Location.StartOffset) == ln && x.Arguments == nil {
				call = x
				return true
			}
		}
		return false
	})
	if call == nil || target == nil {
		return
	}
	body, ok := call.Block.(*parser.BlockNode).Body.(*parser.StatementsNode)
	if !ok {
		return
	}
	if seedsFromLocals(body) {
		return // `subclass.class_eval { @x = x_local }` in a hook: run where it stands (genIvarsEval), not as the class's body
	}
	if c.blockEvalSites == nil {
		c.blockEvalSites = map[*parser.CallNode]bool{}
	}
	c.blockEvalSites[call] = true
	c.collectBody(ctx, f, target, body, c.lexicalOf(target), true)
}

// seedsFromLocals is a block of instance variable writes at least one of which reads an enclosing local (ruby-macho's
// inherited hook copying its class-level state to the subclass): the values exist only where the hook runs.
func seedsFromLocals(body *parser.StatementsNode) bool {
	reads := false
	for _, st := range body.Body {
		w, ok := st.(*parser.InstanceVariableWriteNode)
		if !ok {
			return false
		}
		if _, isLocal := w.Value.(*parser.LocalVariableReadNode); isLocal {
			reads = true
		}
	}
	return reads
}
