package compiler

import (
	"os"
	"regexp"
	"testing"
)

// TestFFITypesMatchPrelude keeps ffiTypes (read at compile time) equal to prelude/ffi.rb's TYPES (used at run time).
func TestFFITypesMatchPrelude(t *testing.T) {
	src, err := os.ReadFile("../../prelude/ffi.rb")
	if err != nil {
		t.Fatal(err)
	}
	fiddleConst := map[string]int{
		"VOID": 0, "VOIDP": 1, "CHAR": 2, "UCHAR": -2, "SHORT": 3, "USHORT": -3, "INT": 4, "UINT": -4, "LONG": 5, "ULONG": -5,
		"FLOAT": 7, "DOUBLE": 8, "CONST_STRING": 10, "BOOL": 11, "SIZE_T": -5, "SSIZE_T": 5, "VARIADIC": 9,
	}
	block := regexp.MustCompile(`(?s)TYPES = \{(.*?)\n  \}`).FindSubmatch(src)
	if block == nil {
		t.Fatal("TYPES not found in prelude/ffi.rb")
	}
	seen := 0
	for _, m := range regexp.MustCompile(`(\w+): Fiddle::TYPE_(\w+)`).FindAllStringSubmatch(string(block[1]), -1) {
		want, ok := fiddleConst[m[2]]
		if !ok {
			t.Fatalf("TYPE_%s has no code in this test", m[2])
		}
		if got, ok := ffiTypes[m[1]]; !ok || got != want {
			t.Errorf("ffiTypes[%q] = %d (present %v), prelude TYPES says %d", m[1], got, ok, want)
		}
		seen++
	}
	if seen != len(ffiTypes) {
		t.Errorf("prelude TYPES has %d entries, ffiTypes %d", seen, len(ffiTypes))
	}
}
