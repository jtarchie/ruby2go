// Package rb2go compiles a typed subset of Ruby to Go.
package rb2go

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jtarchie/ruby2go/internal/compiler"
)

// Prelude holds prelude.rb, its require_relatives, prelude/go/*.go (pure-Go helpers embedded directly; see loadPreludeGo), and
// the vendored gems' lib files (gems/<name>/lib, pure Ruby compiled as user code when required: vendoredLibs, decision 328).
// The character class leaves out prelude/go/0_stubs.go, the generated type-checking stubs (decision 154): helper files
// must start with a letter.
//
//go:embed prelude.rb prelude/*.rb prelude/go/[a-z]*.go gems/*/lib/*.rb
var Prelude embed.FS

// GemSigs holds vendored `.rbs` signatures for the gems rb2go compiles
// (decision 160), under sig/gems/<gem>/. A gem program (one that requires a
// gem, so its load phase runs under MRI at compile time, decision 161) reads
// them to type the gem's otherwise-unannotated classes.
//
//go:embed sig/gems/*/*.rbs
var GemSigs embed.FS

// GoVersion is the go directive for the module generated code builds in.
const GoVersion = "1.25.0"

//go:embed go.sum
var goSum string

// GoMod is the go.mod generated code builds in: the runtime modules a program may import (regexp2 for patterns RE2
// cannot match, decision 242; purego for Fiddle, decision 326); Go loads one only when main.go imports it.
const GoMod = "module gen\n\ngo " + GoVersion + "\n\nrequire (\n\tgithub.com/dlclark/regexp2 " + regexp2Version + "\n\tgithub.com/ebitengine/purego " + puregoVersion + "\n)\n"

// regexp2Version and puregoVersion are the pins GoMod and GoSum derive from; TestGoModMatchesRepo holds them to go.mod.
const (
	regexp2Version = "v1.12.0"
	puregoVersion  = "v0.11.2"
)

// GoSum is go.sum for GoMod: its modules' lines from rb2go's own.
func GoSum() string {
	var b strings.Builder
	for line := range strings.Lines(goSum) {
		for _, mod := range []string{"github.com/dlclark/regexp2 " + regexp2Version, "github.com/ebitengine/purego " + puregoVersion} {
			if strings.HasPrefix(line, mod+" ") || strings.HasPrefix(line, mod+"/go.mod ") {
				b.WriteString(line)
			}
		}
	}
	return b.String()
}

// WriteModule writes GoMod and GoSum into dir.
func WriteModule(dir string) error {
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(GoMod), 0o600)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "go.sum"), []byte(GoSum()), 0o600)
	}
	if err != nil {
		return fmt.Errorf("generated module: %w", err)
	}
	return nil
}

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

// CompileTraced compiles an entry file listing the requires MRI loaded (decision 197), then the program's main file
// (the CLI's -main, decision 288). The -I dirs are gem dirs.
func CompileTraced(ctx context.Context, entryName string, src []byte, main string, loadPath ...string) ([]byte, []string, error) {
	srcs := []compiler.Source{{Name: filepath.Base(entryName), Src: src, Path: entryName}}
	out, warnings, err := compiler.CompileTraced(ctx, Prelude, GemSigs, srcs, main, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rb2go: %w", err)
	}
	return out, warnings, nil
}

// CompileTrace runs main under MRI once per args in runs (rubyOpts first) and compiles what those runs loaded
// (decision 296).
func CompileTrace(ctx context.Context, main string, runs [][]string, rubyOpts []string, loadPath ...string) ([]byte, []string, error) {
	out, warnings, err := compiler.CompileTrace(ctx, Prelude, GemSigs, main, runs, rubyOpts, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rb2go: %w", err)
	}
	return out, warnings, nil
}

// CompileWithGems is Compile for the CLI: a require only an installed gem answers is looked up under MRI and the program compiled with the gem (decision 173); a program without one never starts Ruby.
func CompileWithGems(ctx context.Context, mainName string, src []byte, loadPath ...string) ([]byte, []string, error) {
	srcs := []compiler.Source{{Name: filepath.Base(mainName), Src: src, Path: mainName}}
	out, warnings, err := compiler.CompileWithGems(ctx, Prelude, GemSigs, srcs, loadPath...)
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

// Inference carries parameter types inferred from use (docs/design.md
// decision 146) between compiles of nearly the same sources.
type Inference = compiler.Inference

// NewInference is an empty Inference for CompileTestsSkipping.
func NewInference() *Inference { return compiler.NewInference() }

// CompileTestsSkipping is CompileFiles for running what a test suite can: each test_ method (an `it` included) that fails to compile becomes a minitest skip carrying the error instead of failing the build.
// seed, when not nil, carries inferred parameter types to the next call for the same sources, which then starts from them.
func CompileTestsSkipping(ctx context.Context, files []File, seed *Inference) ([]byte, []string, []SkippedTest, error) {
	srcs := make([]compiler.Source, len(files))
	for i, f := range files {
		srcs[i] = compiler.Source{Name: f.Name, Src: f.Src}
	}
	out, warnings, skipped, err := compiler.CompileTestsSkipping(ctx, Prelude, srcs, seed)
	if err != nil {
		return nil, warnings, skipped, fmt.Errorf("rb2go: %w", err)
	}
	return out, warnings, skipped, nil
}
