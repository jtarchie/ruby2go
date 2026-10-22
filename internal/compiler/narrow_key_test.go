package compiler

import "testing"

func TestNarrowKeys(t *testing.T) {
	if got := ivarNarrowKey("@x"); got != "ivar:@x" {
		t.Errorf("ivarNarrowKey = %q", got)
	}
	if got := attrNarrowKey("x"); got != "attr:x" {
		t.Errorf("attrNarrowKey = %q", got)
	}
	if !isIvarNarrowKey(ivarNarrowKey("@x")) || isIvarNarrowKey(attrNarrowKey("x")) || isIvarNarrowKey("x") {
		t.Error("isIvarNarrowKey must accept only ivar keys")
	}
	if ivarNarrowKey("@x") == attrNarrowKey("x") {
		t.Error("ivar and attr keys collide")
	}
}
