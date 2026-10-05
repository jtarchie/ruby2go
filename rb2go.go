// Package rb2go compiles a typed subset of Ruby to Go.
package rb2go

import (
	"context"
	"embed"
	"fmt"

	"github.com/jtarchie/ruby2go/internal/compiler"
)

// Prelude holds prelude.rb, its require_relatives, and prelude/go/*.go (pure-Go helpers embedded directly; see loadPreludeGo).
//
//go:embed prelude.rb prelude/*.rb prelude/go/*.go
var Prelude embed.FS

// GoVersion is the go directive for the module generated code builds in.
const GoVersion = "1.24"

// Compile transpiles the Ruby source of mainName (with the embedded prelude)
// into a single Go file. loadPath is `ruby -I`'s: directories a `require`
// searches for the program's own files.
func Compile(ctx context.Context, mainName string, src []byte, loadPath ...string) ([]byte, []string, error) {
	out, warnings, err := compiler.CompileWithWarnings(ctx, Prelude, mainName, src, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rb2go: %w", err)
	}
	return out, warnings, nil
}

// File is one Ruby source file of a multi-file program.
type File struct {
	Name string // as given, relative to where the program runs: it is what messages and failure locations show
	Src  []byte
}

// CompileFiles transpiles several Ruby files into one Go program, as Ruby
// loads them into one process: in order, each file's top level running
// before the next's (decision 84).
func CompileFiles(ctx context.Context, files []File, loadPath ...string) ([]byte, []string, error) {
	srcs := make([]compiler.Source, len(files))
	for i, f := range files {
		srcs[i] = compiler.Source{Name: f.Name, Src: f.Src}
	}
	out, warnings, err := compiler.CompileFilesWithWarnings(ctx, Prelude, srcs, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rb2go: %w", err)
	}
	return out, warnings, nil
}

// SkippedTest is a test method CompileTestsSkipping could not compile and turned into a skip.
type SkippedTest = compiler.SkippedTest

// CompileTestsSkipping is CompileFiles for running what a test suite can: each test_ method (an `it` included) that fails to compile becomes a minitest skip carrying the error instead of failing the build.
func CompileTestsSkipping(ctx context.Context, files []File) ([]byte, []string, []SkippedTest, error) {
	srcs := make([]compiler.Source, len(files))
	for i, f := range files {
		srcs[i] = compiler.Source{Name: f.Name, Src: f.Src}
	}
	out, warnings, skipped, err := compiler.CompileTestsSkipping(ctx, Prelude, srcs)
	if err != nil {
		return nil, warnings, skipped, fmt.Errorf("rb2go: %w", err)
	}
	return out, warnings, skipped, nil
}
