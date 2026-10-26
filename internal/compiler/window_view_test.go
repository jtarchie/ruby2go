package compiler

import (
	"context"
	"testing"
)

// windowBlockSafe is what decides decision 384's reuse; the minitest suite passes either way, so this pins which blocks get it.
func TestWindowBlockSafe(t *testing.T) {
	p, err := sharedParser()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		block     string
		immediate bool
		safe      bool
	}{
		{"{ n += 1 }", true, true},
		{"{ |a, b, c| kept << b }", false, true},
		{"{ |(a, b), c| kept << a }", false, true},
		{"{ |w| w.sum % 5 == 0 }", true, true},
		{"{ |w| w.sum(&:age) }", false, true},
		{"{ |w| kept << w.dup }", true, true},
		{"{ |w| w.first(2) }", false, true},
		{"{ |w| w.first }", true, true},
		{"{ |w| w.first }", false, false},
		{"{ |w| w[0] + 1 }", false, true},
		{"{ |w| total += w.last.to_i }", false, true},
		{"{ |w| next if w.first; n += 1 }", false, true},
		{"{ |w| w.first; n += 1 }", false, true},
		{"{ |w| kept << w.first }", true, false},
		{"{ |w| kept << w }", true, false},
		{"{ |w| w }", true, false},
		{"{ |w| w << 1 }", true, false},
		{"{ |w| w.to_a }", true, false},
		{"{ |w| w = [1]; w.sum }", true, false},
		{"{ |w| procs << -> { w.sum } }", true, false},
		{"{ |w| w.each { |x| p w.sum } }", true, false},
		{"{ |w| binding }", true, false},
		{"{ |w| \"#{w}\" }", true, false},
		{"{ |a, *rest| a }", true, false},
		{"{ |a,| a }", true, false},
		{"{ it.sum }", true, false},
		{"{ _1.sum }", true, false},
	} {
		res, err := p.Parse(context.Background(), []byte("xs.each_cons(2) "+c.block+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		b := firstBlock(res.Value)
		if b == nil {
			t.Fatalf("%s: no block", c.block)
		}
		if got := windowBlockSafe(b, c.immediate); got != c.safe {
			t.Errorf("%s (immediate %v): safe = %v, want %v", c.block, c.immediate, got, c.safe)
		}
	}
}
