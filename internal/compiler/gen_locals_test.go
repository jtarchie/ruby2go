package compiler

import (
	"context"
	"testing"

	parser "github.com/danielgatis/go-ruby-prism/parser"
)

func firstBlock(n parser.Node) *parser.BlockNode {
	if b, ok := n.(*parser.BlockNode); ok {
		return b
	}
	for _, ch := range n.CompactChildNodes() {
		if b := firstBlock(ch); b != nil {
			return b
		}
	}
	return nil
}

// A closure made before a block param's reassignment that writes it by any write form captures it: renameReassignedParams must not rename (the closure and the later reads share one variable).
func TestRenameReassignedParamsCapture(t *testing.T) {
	p, err := sharedParser()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, closure string
		renamed       bool
	}{
		{"none", "x = 1", true},
		{"read", "c = -> { p }", false},
		{"write", "c = -> { p = 2 }", false},
		{"op-write", "c = -> { p += 1 }", false},
		{"or-write", "c = -> { p ||= 1 }", false},
		{"and-write", "c = -> { p &&= 1 }", false},
		{"multi-write", "c = -> { p, q = 1, 2 }", false},
		{"for-target", "c = -> { for p in [1]; end }", false},
	} {
		src := "foo { |p|\n  " + c.closure + "\n  p = p.to_s\n  p\n}\n"
		res, err := p.Parse(context.Background(), []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		b := firstBlock(res.Value)
		renameReassignedParams(b, []string{"p"})
		if got := len(b.Locals) > 0 && b.Locals[len(b.Locals)-1] == "p__r1"; got != c.renamed {
			t.Errorf("%s: renamed = %v, want %v (locals %v)", c.name, got, c.renamed, b.Locals)
		}
	}
}
