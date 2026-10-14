package boot

import (
	"context"
	"path/filepath"
	"testing"
)

// A captured Float local is recorded as a Ruby literal: NaN and Infinity inspect as bare words the compiler would read as constants.
func TestCaptureNonFiniteFloat(t *testing.T) {
	needRuby(t)
	dir := writeEntry(t, "entry.rb", `class K
  [Float::NAN, Float::INFINITY, -Float::INFINITY, 1.5].each_with_index do |v, i|
    define_method(:"m#{i}") { v }
  end
end
`)
	m, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range m.DefinedMethods("K") {
		got[d.Name] = d.Captures["v"]
	}
	want := map[string]string{"m0": "Float::NAN", "m1": "Float::INFINITY", "m2": "-Float::INFINITY", "m3": "1.5"}
	for name, lit := range want {
		if got[name] != lit {
			t.Errorf("%s captured v = %q, want %q", name, got[name], lit)
		}
	}
}
