package rb2go

import (
	"context"
	"strings"
	"testing"
)

// openMemoProgram's hashes and arrays carry no annotation: what is stored in them types them (decisions 182, 344).
const openMemoProgram = `nums = [1, 2, 3, 4, 5]
h = Hash.new(0)
nums.each { |x| h[x % 2] += 1 }
g = nums.each_with_object(Hash.new(0)) { |n, acc| acc[n % 3] += 1 }
e = nums.each_with_object({}) { |n, acc| acc[n] = n.to_s }
l = nums.each_with_object([]) { |n, acc| acc << n * 2 }
t = nums.inject(Hash.new(0)) { |acc, n| acc[n.even?] += 1; acc }
p h, g, e, l, t
`

func TestOpenMemoTyped(t *testing.T) {
	t.Parallel()
	out, warns, err := Compile(context.Background(), "main.rb", []byte(openMemoProgram))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warns {
		if strings.Contains(w, "untyped") {
			t.Errorf("warning: %s", w)
		}
	}
	src := string(out)
	for _, want := range []string{
		"h := Hash_Meta_New[Hash_MetaI, Integer, Integer](",
		"EachWithObject[*Array[Integer], Integer, *Hash[Integer, Integer]](",
		"EachWithObject[*Array[Integer], Integer, *Hash[Integer, String]](",
		"EachWithObject[*Array[Integer], Integer, *Array[Integer]](",
		"Inject[*Array[Integer], Integer, *Hash[Boolean, Integer]](",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("main.go lacks %s", want)
		}
	}
}
