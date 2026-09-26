// Package compiler turns typed Ruby (rbs-inline) into Go.
package compiler

import (
	"context"
	"fmt"
	"os"
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

// Compile transpiles prelude + main source into one Go file.
func Compile(ctx context.Context, preludeSrc []byte, mainName string, mainSrc []byte) ([]byte, error) {
	out, _, err := CompileWithWarnings(ctx, preludeSrc, mainName, mainSrc)
	return out, err
}

func compile(ctx context.Context, preludeSrc []byte, mainName string, mainSrc []byte, warnings *[]string) (out []byte, err error) {
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
		return nil, err
	}
	defer p.Close(ctx)

	c := &Compiler{classes: map[string]*Class{}, topDefs: map[string]*Method{}, tupleN: map[int]bool{}}
	pf, err := parseFile(ctx, p, "prelude.rb", preludeSrc, true)
	if err != nil {
		return nil, err
	}
	mf, err := parseFile(ctx, p, filepath.Base(mainName), mainSrc, false)
	if err != nil {
		return nil, err
	}
	c.files = []*File{pf, mf}
	c.mainFile = mf
	c.collect(pf)
	if len(c.mainStmts) > 0 {
		c.errorf(pf, c.mainStmts[0], "prelude must not have top-level statements")
	}
	c.collect(mf)
	c.link()
	c.discoverIvars()
	c.emitProgram()
	*warnings = c.Warnings
	src := []byte(c.out.String())
	formatted, ferr := formatGo(src)
	if ferr != nil {
		// Keep the raw output around for debugging.
		tmp := filepath.Join(os.TempDir(), "rb2go-bad-output.go")
		_ = os.WriteFile(tmp, src, 0o644)
		return nil, fmt.Errorf("internal error: generated Go does not parse (%v); raw output written to %s", ferr, tmp)
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
func CompileWithWarnings(ctx context.Context, preludeSrc []byte, mainName string, mainSrc []byte) ([]byte, []string, error) {
	var warnings []string
	out, err := compile(ctx, preludeSrc, mainName, mainSrc, &warnings)
	return out, warnings, err
}
