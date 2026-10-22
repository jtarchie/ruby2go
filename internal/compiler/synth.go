package compiler

import (
	"context"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// synthOpt says how a generated source is placed against the declaration that asked for it.
type synthOpt struct {
	Origin Origin
	Pad    int  // the generated text reports lines Pad further down, as if Pad newlines led it (without parsing them)
	Pin    bool // every position's shown line (errors, //line) is the declaration's
	NoPath bool // leave the path empty: gemFile then says no, so even in gem code the generated source is user code
}

// synthFile parses Ruby generated for the declaration at in f; it keeps f's path so gem code stays gem code (decision 172).
func (c *Compiler) synthFile(ctx context.Context, f *File, at parser.Node, label, src string, o synthOpt) *File {
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), o.Origin)
	if err != nil {
		c.errorf(f, at, "%s: %v\n%s", label, err, src)
	}
	if !o.NoPath {
		sf.path = f.path
	}
	if o.Pin {
		sf.pinLine = f.line(at.GetLocation().StartOffset)
	}
	sf.shiftLines(o.Pad)
	return sf
}

// synthDef is synthFile for a source whose first statement is the def to add: it returns that def.
func (c *Compiler) synthDef(ctx context.Context, f *File, at parser.Node, label, src string, o synthOpt) (*File, *parser.DefNode) {
	sf := c.synthFile(ctx, f, at, label, src, o)
	return sf, c.synthDefs(f, at, label, sf)[0]
}

// synthDefs are sf's top-level statements, each checked to be a def.
func (c *Compiler) synthDefs(f *File, at parser.Node, label string, sf *File) []*parser.DefNode {
	body := sf.Root.Statements.Body
	defs := make([]*parser.DefNode, 0, len(body))
	for _, st := range body {
		d, ok := st.(*parser.DefNode)
		if !ok {
			c.errorf(f, at, "%s: generated source has %s where a def belongs", label, nodeType(st))
		}
		defs = append(defs, d)
	}
	if len(defs) == 0 {
		c.errorf(f, at, "%s: generated source has no def", label)
	}
	return defs
}

// shiftLines is n leading newlines without parsing them: line starts and the comment tables are all keyed by line.
func (f *File) shiftLines(n int) {
	if n <= 0 {
		return
	}
	f.lines = append(make([]int, n), f.lines...)
	f.leading = shiftKeys(f.leading, n, func(cs []comment) []comment {
		for i := range cs {
			cs[i].line += n
		}
		return cs
	})
	f.trailing = shiftKeys(f.trailing, n, func(s string) string { return s })
	f.typeArgs = shiftKeys(f.typeArgs, n, func(s string) string { return s })
}

func shiftKeys[V any](m map[int]V, n int, fix func(V) V) map[int]V {
	out := make(map[int]V, len(m))
	for k, v := range m {
		out[k+n] = fix(v)
	}
	return out
}
