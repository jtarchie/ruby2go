package compiler

import (
	"os"
	"regexp"
	"testing"
)

// Two files with an END block at the same byte offset must each register their own at_exit.
func TestEndBlocksSameOffsetTwoFiles(t *testing.T) {
	srcs := []Source{
		{Name: "a.rb", Src: []byte("END { puts \"a\" }\n")},
		{Name: "b.rb", Src: []byte("END { puts \"b\" }\n")},
	}
	out, _, err := CompileFilesWithWarnings(t.Context(), os.DirFS("../.."), srcs)
	if err != nil {
		t.Fatal(err)
	}
	guards := regexp.MustCompile(`(?m)^var rbEnd\w+ bool$`).FindAllString(string(out), -1)
	if len(guards) != 2 {
		t.Errorf("want one guard per END block, got %q", guards)
	}
}
