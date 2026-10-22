package boot

import (
	"encoding/json"
	"testing"
)

// A record's JSON is what boot_hook.rb writes: file/line (and col) stay flat keys in the same order, whatever Go type holds them.
func TestRecordJSONShape(t *testing.T) {
	cases := []struct {
		name string
		in   string
		into any
	}{
		{"alias", `{"owner":"A","singleton":false,"name":"n","old":"o","file":"/a/b.rb","line":7}`, new(AliasInfo)},
		{"include", `{"owner":"A","site":"/a/b.rb:3","module":"M","file":"/a/b.rb","line":9}`, new(IncludeInfo)},
		{"branch", `{"file":"/a/b.rb","line":4,"col":0,"kind":"if","then":1,"else":0,"arms":[{"kind":"when","line":5,"col":2,"count":1}]}`, new(BranchInfo)},
		{"method", `{"owner":"A","singleton":false,"name":"m","file":"/a/b.rb","line":2,"source":"","from":"","private":false,"module_function":false}`, new(MethodInfo)},
		{"eval", `{"file":"/a/b.rb","line":6,"source":"x","caller":"/a/c.rb:1","kind":"eval","self":""}`, new(EvalInfo)},
	}
	for _, tc := range cases {
		err := json.Unmarshal([]byte(tc.in), tc.into)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		out, err := json.Marshal(tc.into)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if string(out) != tc.in {
			t.Errorf("%s: JSON changed\n got %s\nwant %s", tc.name, out, tc.in)
		}
	}
}

func TestLocStringParse(t *testing.T) {
	for _, l := range []Loc{{"/a/b.rb", 12}, {"/gems/odd:dir/x.rb", 3}, {"", 0}} {
		got, ok := ParseLoc(l.String())
		if !ok || got != l {
			t.Errorf("ParseLoc(%q) = %v, %v; want %v", l.String(), got, ok, l)
		}
	}
	for _, s := range []string{"", "x.rb", "x.rb:abc", "x.rb:"} {
		if _, ok := ParseLoc(s); ok {
			t.Errorf("ParseLoc(%q) ok, want not", s)
		}
	}
}
