// Package compiler turns typed Ruby (rbs-inline) into Go.
package compiler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Compiler holds the whole program: the prelude and the user files, one closed world.
type Compiler struct {
	classes       map[string]*Class
	classList     []*Class
	topDefs       map[string]*Method
	consts        map[string]*Const
	topConstNames []string // constants declared at top level: Object's table
	dynNames      []string // method names called on untyped values
	dynSeen       map[string]bool
	dynAll        bool     // a computed send/respond_to?: every method may be named
	respondNames  []string // names asked about with respond_to? at run time
	respondSeen   map[string]bool
	markers       map[string]bool         // rbHas<marker> helpers emitted
	classOf       bool                    // `.class` on a value only known at run time: emit rbClassOf
	dynGo         map[string]string       // Go name → the Ruby name its dispatchers serve
	dynWrapped    map[*Class][]dynWrapped // wrappers emitted, per class, for _Call
	warned        map[string]bool
	regexps       []string          // package-level compiled literals
	regexpVars    map[string]string // literal → its variable, to share one per pattern
	strLits       map[string]bool   // String literal texts: String#frozen? knows them by identity
	constList     []*Const
	hooks         []classHook // main.rb's inherited/included/extended sites
	topDefList    []*Method
	verbatim      []verbatim
	mainStmts     []parser.Node
	mainFile      *File   // the first user file: $0, and the generated header
	userFiles     []*File // in load order
	stmtFile      map[parser.Node]*File
	files         []*File
	preludeFS     fs.FS
	parser        *parser.Parser
	loaded        map[string]bool
	out           strings.Builder
	convs         map[string]bool // conversion sites emitted during a refineIvars dry run
	Warnings      []string
	// tuple arities used, so their types get emitted
	tupleN      map[int]bool
	procTypes   map[string]bool // Proc Go types rendered (*func(...)), for the generated rbIsProc
	argBoxes    map[string]bool // T? boxes of generic type arguments, which generic code may hold
	classIDs    map[*Class]int  // index in classList: the class's ID in the generated tables
	specClasses int             // describes declared, for their classes' Go names
	specUses    []specUse       // Minitest::Spec DSL calls, checked after link
	// concrete T? Go types (*T) rendered anywhere, for rbUnbox; the value
	// says whether T is itself optional
	boxes map[string]bool
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

// sharedParser is one Prism pool per process: instantiating the WASM module costs ~0.5s, more than the rest of a compile.
var sharedParser = sync.OnceValues(func() (*parser.Parser, error) {
	return parser.NewParser(context.Background(), parser.WithVersion(parser.SyntaxVersionLatest), parser.WithPoolSize(runtime.GOMAXPROCS(0)))
})

// Compile transpiles the prelude (prelude.rb in preludeFS, plus whatever it
// require_relatives) and the main source into one Go file.
func Compile(ctx context.Context, preludeFS fs.FS, mainName string, mainSrc []byte) ([]byte, error) {
	out, _, err := CompileWithWarnings(ctx, preludeFS, mainName, mainSrc)
	return out, err
}

// Source is one user file: its name as messages and //line show it, and its text.
type Source struct {
	Name string
	Src  []byte
}

func compile(ctx context.Context, preludeFS fs.FS, sources []Source, warnings *[]string) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ce, ok := r.(compileError); ok {
				err = ce
				return
			}
			panic(r)
		}
	}()
	p, err := sharedParser()
	if err != nil {
		return nil, fmt.Errorf("prism: %w", err)
	}

	c := &Compiler{classes: map[string]*Class{}, topDefs: map[string]*Method{}, consts: map[string]*Const{}, tupleN: map[int]bool{}, procTypes: map[string]bool{}, argBoxes: map[string]bool{}, boxes: map[string]bool{}, regexpVars: map[string]string{}, strLits: map[string]bool{}, dynSeen: map[string]bool{}, respondSeen: map[string]bool{}, markers: map[string]bool{}, dynGo: map[string]string{}, dynWrapped: map[*Class][]dynWrapped{}, warned: map[string]bool{},
		preludeFS: preludeFS, parser: p, loaded: map[string]bool{}}
	c.loadPreludeGo()
	c.loadPrelude(ctx, "prelude.rb")
	for _, src := range sources {
		uf, perr := parseFile(ctx, p, src.Name, src.Src, false)
		if perr != nil {
			return nil, perr
		}
		c.files = append(c.files, uf)
		c.userFiles = append(c.userFiles, uf)
		c.collect(ctx, uf)
	}
	c.mainFile = c.userFiles[0]
	c.nameGo()
	c.link(ctx)
	c.discoverIvars()
	c.refineIvars()
	c.inferReturns()
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
	return CompileFilesWithWarnings(ctx, preludeFS, []Source{{Name: filepath.Base(mainName), Src: mainSrc}})
}

// CompileFilesWithWarnings compiles several user files into one program, loaded in order (decision 84).
func CompileFilesWithWarnings(ctx context.Context, preludeFS fs.FS, sources []Source) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	var warnings []string
	out, err := compile(ctx, preludeFS, sources, &warnings)
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

// loadPreludeGo embeds prelude/go/*.go verbatim: pure Go with no self/param binding, so it skips Ruby parsing entirely.
func (c *Compiler) loadPreludeGo() {
	names, err := fs.Glob(c.preludeFS, "prelude/go/*.go")
	if err != nil {
		panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
	}
	sort.Strings(names)
	for _, name := range names {
		src, err := fs.ReadFile(c.preludeFS, name)
		if err != nil {
			panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
		}
		body, line := stripGoPackage(src)
		c.verbatim = append(c.verbatim, verbatim{file: &File{Name: name, prelude: true}, line: line, code: body})
	}
}

// stripGoPackage drops the file header comment and `package` clause, keeping only what follows; line is where that remainder starts, for the //line directive.
func stripGoPackage(src []byte) (string, int) {
	lines := strings.Split(string(src), "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "package ") {
			return strings.Join(lines[i+1:], "\n"), i + 2
		}
	}
	return string(src), 1
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

// warn records a warning once, with its source position.
func (c *Compiler) warn(f *File, n parser.Node, format string, args ...any) {
	msg := fmt.Sprintf("%s:%d: ", f.Name, f.line(n.GetLocation().StartOffset)) + fmt.Sprintf(format, args...)
	if c.warned[msg] {
		return
	}
	c.warned[msg] = true
	c.Warnings = append(c.Warnings, msg)
}

// dropWarnings forgets the warnings after the first n, for a re-run pass.
func (c *Compiler) dropWarnings(n int) {
	for _, w := range c.Warnings[n:] {
		delete(c.warned, w)
	}
	c.Warnings = c.Warnings[:n]
}

// noteDyn asks for dynamic wrappers of a method name.
func (c *Compiler) noteDyn(name string) {
	if c.dynSeen[name] {
		return
	}
	c.dynSeen[name] = true
	c.dynNames = append(c.dynNames, name)
	c.noteDyn("method_missing")
}

// noteRespond asks for a run-time respond_to? check of a method name.
func (c *Compiler) noteRespond(name string) {
	if c.respondSeen[name] {
		return
	}
	c.respondSeen[name] = true
	c.respondNames = append(c.respondNames, name)
	c.noteDyn("respond_to_missing?")
}
