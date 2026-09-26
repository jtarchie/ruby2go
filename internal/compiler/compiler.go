// Package compiler turns typed Ruby (rbs-inline) into Go.
package compiler

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Compiler holds the whole program: prelude + one user file.
type Compiler struct {
	classes    map[string]*Class
	classList  []*Class
	topDefs    map[string]*Method
	topDefList []*Method
	verbatim   []verbatim
	mainStmts  []parser.Node
	mainFile   *File
	files      []*File
	preludeFS  fs.FS
	parser     *parser.Parser
	loaded     map[string]bool
	out        strings.Builder
	Warnings   []string
	// tuple arities used, so their types get emitted
	tupleN map[int]bool
}

type verbatim struct {
	file *File
	line int
	code string
}

type compileError struct{ msg string }

func (e compileError) Error() string { return e.msg }

// errorf aborts compilation. If n is non-nil the message is prefixed with
// file:line.
func (c *Compiler) errorf(f *File, n parser.Node, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if n != nil && f != nil {
		msg = fmt.Sprintf("%s:%d: %s", f.Name, f.line(n.GetLocation().StartOffset), msg)
	}
	panic(compileError{msg: msg})
}

func (c *Compiler) unsupported(f *File, n parser.Node) {
	c.errorf(f, n, "unsupported syntax: %s", nodeType(n))
}

func nodeType(n parser.Node) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", n), "*parser.")
}

// Compile transpiles the prelude (prelude.rb in preludeFS, plus whatever it
// require_relatives) and the main source into one Go file.
func Compile(ctx context.Context, preludeFS fs.FS, mainName string, mainSrc []byte) ([]byte, error) {
	out, _, err := CompileWithWarnings(ctx, preludeFS, mainName, mainSrc)
	return out, err
}

func compile(ctx context.Context, preludeFS fs.FS, mainName string, mainSrc []byte, warnings *[]string) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ce, ok := r.(compileError); ok {
				err = ce
				return
			}
			panic(r)
		}
	}()
	p, err := parser.NewParser(ctx, parser.WithVersion(parser.SyntaxVersionLatest), parser.WithPoolSize(1))
	if err != nil {
		return nil, fmt.Errorf("prism: %w", err)
	}
	defer func() { _ = p.Close(ctx) }()

	c := &Compiler{classes: map[string]*Class{}, topDefs: map[string]*Method{}, tupleN: map[int]bool{},
		preludeFS: preludeFS, parser: p, loaded: map[string]bool{}}
	c.loadPrelude(ctx, "prelude.rb")
	mf, err := parseFile(ctx, p, filepath.Base(mainName), mainSrc, false)
	if err != nil {
		return nil, err
	}
	c.files = append(c.files, mf)
	c.mainFile = mf
	c.collect(ctx, mf)
	c.link()
	c.discoverIvars()
	c.emitProgram()
	*warnings = c.Warnings
	src := []byte(c.out.String())
	formatted, ferr := formatGo(src)
	if ferr != nil {
		// Keep the raw output around for debugging.
		tmp := filepath.Join(os.TempDir(), "rb2go-bad-output.go")
		_ = os.WriteFile(tmp, src, 0o600)
		return nil, fmt.Errorf("internal error: generated Go does not parse (%w); raw output written to %s", ferr, tmp)
	}
	return formatted, nil
}

// sortedClasses returns classes in a stable emission order: prelude order
// first (as declared), then user classes.
func (c *Compiler) sortedClasses() []*Class {
	out := append([]*Class(nil), c.classList...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].File.prelude && !out[j].File.prelude
	})
	return out
}

// CompileWithWarnings is Compile plus the warnings collected.
func CompileWithWarnings(ctx context.Context, preludeFS fs.FS, mainName string, mainSrc []byte) ([]byte, []string, error) {
	var warnings []string
	out, err := compile(ctx, preludeFS, mainName, mainSrc, &warnings)
	return out, warnings, err
}

// loadPrelude parses and collects one prelude file. `require_relative` at
// its top level pulls in further files, in place, so emission order follows
// require order.
func (c *Compiler) loadPrelude(ctx context.Context, name string) {
	if c.loaded[name] {
		return
	}
	c.loaded[name] = true
	src, err := fs.ReadFile(c.preludeFS, name)
	if err != nil {
		panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
	}
	f, err := parseFile(ctx, c.parser, name, src, true)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	c.files = append(c.files, f)
	c.collect(ctx, f)
	if len(c.mainStmts) > 0 {
		c.errorf(f, c.mainStmts[0], "prelude must not have top-level statements")
	}
}

// requireRelative resolves `require_relative "x"` inside prelude file f.
func (c *Compiler) requireRelative(ctx context.Context, f *File, n *parser.CallNode) {
	args := callArgs(n)
	str, ok := args[0].(*parser.StringNode)
	if len(args) != 1 || !ok {
		c.errorf(f, n, "require_relative needs a string literal")
	}
	target := path.Join(path.Dir(f.Name), str.Unescaped.Value)
	if !strings.HasSuffix(target, ".rb") {
		target += ".rb"
	}
	c.loadPrelude(ctx, target)
}
