package rugaroo

import (
	"context"
	"strings"
	"testing"
)

// A program that loads WeakRef gives each root struct class a pointer field, so Go never batches its objects into one
// tiny-allocator slot, where a weak pointer may never clear (decision 351); one that does not keeps its layout.
func TestWeakRefReferentsNotTiny(t *testing.T) {
	t.Parallel()
	const body = "class Box\n  attr_reader :n #: Integer\n\n  #: (Integer) -> void\n  def initialize(n)\n    @n = n\n  end\nend\n\np Box.new(1).n\n"
	for _, tc := range []struct {
		src  string
		want bool
	}{{"require \"weakref\"\n" + body + "p WeakRef.new(Box.new(2)).weakref_alive?\n", true}, {body, false}} {
		out, _, err := Compile(context.Background(), "main.rb", []byte(tc.src))
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		i := strings.Index(s, "type Box struct {")
		if i < 0 {
			t.Fatal("no Box struct")
		}
		decl := s[i : i+strings.Index(s[i:], "}")]
		if got := strings.Contains(decl, "_ *byte"); got != tc.want {
			t.Errorf("weakref loaded=%v: Box is %q", tc.want, decl)
		}
	}
}
