package compiler

import (
	"strings"
	"testing"
)

// MRI has one @v: a module reading what the includer's superclass assigns in any write form must read it by name (decision 324).
func TestIncluderClassWritesAnyForm(t *testing.T) {
	for name, write := range map[string]string{
		"plain":  "@v = 5",
		"or":     "@v ||= 5",
		"and":    "@v &&= 5",
		"target": "@v, @w = 5, 6",
	} {
		t.Run(name, func(t *testing.T) {
			gem := "class Seg\n  def init\n    " + write + "\n  end\nend\nmodule Shower\n  def show = @v\nend\nclass Holder < Seg\n  include Shower\nend\n"
			out, err := compileGemApp(t, map[string]string{"thing.rb": gem}, "require \"thing\"\nh = Holder.new\nh.init\nputs h.show\n")
			if err != nil {
				t.Fatal(err)
			}
			_, body, ok := strings.Cut(out, "func Shower_Show")
			if !ok {
				t.Fatal("Shower_Show not emitted")
			}
			body, _, _ = strings.Cut(body, "\n}\n")
			if !strings.Contains(body, `rbIvarGet(self, Symbol("@v"))`) {
				t.Errorf("Shower_Show does not read the includer's @v by name:\n%s", body)
			}
		})
	}
}
