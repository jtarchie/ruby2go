package compiler

import "testing"

func TestSrcPosString(t *testing.T) {
	for _, tc := range []struct {
		pos  srcPos
		want string
	}{
		{srcPos{}, ""},
		{srcPos{&File{Name: "main.rb"}, 3}, "main.rb:3"},
		{srcPos{&File{Name: "a:b c.rb"}, 12}, "a:b c.rb:12"},
	} {
		if got := tc.pos.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.pos, got, tc.want)
		}
	}
}

// errorAt keeps the location apart from the message, so a file name with ':' or spaces needs no parsing to take it off.
func TestErrorAt(t *testing.T) {
	at := func(pos srcPos) (ce compileError) {
		defer func() { ce = recover().(compileError) }()
		(&Compiler{}).errorAt(pos, "bad %s", "thing")
		return
	}
	for _, tc := range []struct {
		pos       srcPos
		text, msg string
	}{
		{srcPos{}, "bad thing", "bad thing"},
		{srcPos{&File{Name: "main.rb"}, 4}, "main.rb:4: bad thing", "bad thing"},
		{srcPos{&File{Name: "a:b c.rb"}, 7}, "a:b c.rb:7: bad thing", "bad thing"},
	} {
		ce := at(tc.pos)
		if ce.Error() != tc.text || ce.msg != tc.msg {
			t.Errorf("errorAt(%q) = %q (msg %q), want %q (msg %q)", tc.pos, ce.Error(), ce.msg, tc.text, tc.msg)
		}
	}
}
