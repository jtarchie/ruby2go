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
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Compiler holds the whole program: the prelude and the user files, one closed world.
type Compiler struct {
	erbSnippets   map[*parser.CallNode]*File  // compiled ERB templates by their result call (decision 111)
	rewrites      map[parser.Node]parser.Node // desugared nodes, built once so every pass sees one tree
	loopInner     map[*parser.CallNode]bool   // the loop calls loopRescue wrapped, not wrapped again
	erbCounter    int
	labels        map[string]string // Go function (its table key) → Ruby backtrace label, for every function emitted (decision 106)
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
	regexps       []string                // package-level compiled literals (and DATA's StringIO)
	dataVar       bool                    // rbDATA is declared
	beginStmts    map[*File][]parser.Node // BEGIN { } bodies, run first in their file
	regexpVars    map[string]string       // literal → its variable, to share one per pattern
	strLits       map[string]bool         // String literal texts: String#frozen? knows them by identity
	constList     []*Const
	hooks         []classHook // main.rb's inherited/included/extended sites
	topDefList    []*Method
	verbatim      []verbatim
	mainStmts     []parser.Node
	mainFile      *File            // the first user file: $0, and the generated header
	userFiles     []*File          // in load order
	loadCode      map[*File]string // a required file's top level, generated, for the loadFile that splices it
	required      map[*File]bool   // user files require_relative loaded: they run where required, not as a top level of their own
	stmtFile      map[parser.Node]*File
	dynEvery      bool            // emit every noted dispatcher and forwarder up front: computed send is reachable, or a lazy pass came up short
	dynLazy       map[string]bool // names noted before any dispatcher went out: emitted only when reached (emitDynamic)
	dynOut        map[string]bool // dispatchers emitted
	respondOut    map[string]bool // respond_to? checks emitted
	fwdOut        map[*Class]bool // forwarders emitted
	tablesOut     bool
	tablesAt      [9]int          // tableInputs when the tables went out
	callable      map[string]bool // what the _Call tables switch over, once kept code asks for _Call (callableNames)
	files         []*File
	preludeFS     fs.FS
	parser        *parser.Parser
	loaded        map[string]bool // prelude names, and user files' real paths
	loadPath      []string        // -I directories (decision 131)
	out           strings.Builder
	convs         map[string]bool // conversion sites emitted during a refineIvars dry run
	Warnings      []string
	// tuple arities used, so their types get emitted
	tupleN           map[int]bool
	procTypes        map[string]bool                   // Proc Go types rendered (*func(...)), for the generated rbIsProc
	argBoxes         map[string]bool                   // T? boxes of generic type arguments, which generic code may hold
	classIDs         map[*Class]int                    // index in classList: the class's ID in the generated tables
	specClasses      int                               // describes declared, for their classes' Go names
	anonCount        int                               // Class.new/Module.new literals declared (decision 145)
	anonClasses      map[*parser.CallNode]*Class       // each literal's class
	inheritedAliases []pendingAlias                    // aliases of an ancestor's method, copied once supers resolve
	unnamedAnon      map[*parser.CallNode]bool         // Class.new literals no constant names: they run inherited where evaluated
	privateConsts    map[string]bool                   // private_constant's full names: no `M::X` path reaches them
	anonErrors       map[*parser.CallNode]compileError // a literal whose body did not collect: raised where it is generated
	specUses         []specUse                         // Minitest::Spec DSL calls, checked after link
	// concrete T? Go types (*T) rendered anywhere, for rbUnbox; the value
	// says whether T is itself optional
	boxes          map[string]bool
	marshalSeen    map[string]Type // concrete container and tuple types of values, by Ruby type: Marshal's cases (decision 137)
	marshalGo      map[string]bool // their Go types with a case emitted
	marshalSkipped []string        // types left out of Marshal's cases as unreached: reaching one later recompiles eagerly
	marshalCode    string          // rbMDumpGen/rbMLoadGen, rendered when the tables go out
	marshalErr     *compileError   // a bad marshal_dump/marshal_load, raised once Marshal is reached
	marshalAt      int             // len(marshalSeen) then
	marshalOut     bool            // rbMDumpGen/rbMLoadGen emitted
	marshalLate    bool            // rbMDumpObjGen/rbMLoadObjGen emitted
	// parameter types from use (decision 146)
	infer       *inference
	round       bool                  // an inference round: pending parameters are untyped, their arguments recorded
	inferDone   bool                  // the rounds ran: a parameter still untyped is an error
	uses        map[string][]paramUse // this round's arguments, by parameter key
	pendingSeen map[string]bool       // this round's untyped parameter keys
}

type verbatim struct {
	file *File
	line int
	code string
}

type compileError struct {
	msg     string
	untyped bool // a parameter inference left untyped (decision 146): a failed round's own error explains it better
}

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
	Path string // where it is on disk, for require_relative; Name when empty
}

// options are a compile's settings beyond its sources.
type options struct {
	skipTests bool          // a test_ method that fails to compile becomes a skip (SkippedTest) instead of an error
	skipped   []SkippedTest // what skipTests skipped
	warnings  []string
	loadPath  []string // -I directories, which a user `require` searches first
	infer     *inference
	round     bool
	inferDone bool
	seed      *Inference // types from an earlier compile of these sources, updated by this one
}

// SkippedTest is a test method skipTests turned into a minitest skip.
type SkippedTest struct {
	File   string
	Line   int
	Name   string // the Go method's Ruby name: test_0001_desc for an `it`
	Reason string // the compile error, without its file:line
}

func compile(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options) ([]byte, error) {
	if opts.seed != nil {
		opts.infer = opts.seed.inf
	}
	out, err := compileWith(ctx, preludeFS, sources, opts, false)
	if errors.Is(err, errNeedInfer) {
		if opts.infer == nil {
			opts.infer = &inference{}
		}
		if opts.seed != nil {
			defer func() { opts.seed.inf = opts.infer }()
		}
		var roundErr error
		for range maxInferRounds {
			opts.round = true
			_, err = compileWith(ctx, preludeFS, sources, opts, false)
			opts.round = false
			if err != nil && !errors.Is(err, errInferRound) {
				roundErr = err // it stopped inference: the final compile's untyped parameters are its doing
				if os.Getenv("RB2GO_INFER_DEBUG") != "" {
					fmt.Fprintln(os.Stderr, "rb2go: infer round failed:", err)
				}
			}
			if !errors.Is(err, errInferRound) || !opts.infer.changed {
				break
			}
		}
		opts.inferDone = true
		out, err = compileWith(ctx, preludeFS, sources, opts, false)
		var ce compileError
		if roundErr != nil && errors.As(err, &ce) && ce.untyped {
			err = roundErr
		}
	}
	if errors.Is(err, errPruneIncomplete) {
		if os.Getenv("RB2GO_TIMING") != "" {
			fmt.Fprintln(os.Stderr, "rb2go: fallback: recompiling with every dispatcher")
		}
		opts.skipped = nil
		out, err = compileWith(ctx, preludeFS, sources, opts, true)
	}
	return out, err
}

func compileWith(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options, dynEvery bool) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ce, ok := r.(compileError); ok {
				err = ce
				return
			}
			if _, ok := r.(needInfer); ok {
				err = errNeedInfer
				return
			}
			panic(r)
		}
	}()
	tick := phaseTimer()
	p, err := sharedParser()
	if err != nil {
		return nil, fmt.Errorf("prism: %w", err)
	}
	tick("parser")

	c := &Compiler{classes: map[string]*Class{}, topDefs: map[string]*Method{}, consts: map[string]*Const{}, tupleN: map[int]bool{}, procTypes: map[string]bool{}, argBoxes: map[string]bool{}, boxes: map[string]bool{}, marshalSeen: map[string]Type{}, marshalGo: map[string]bool{}, regexpVars: map[string]string{}, strLits: map[string]bool{}, dynSeen: map[string]bool{}, respondSeen: map[string]bool{}, markers: map[string]bool{}, dynGo: map[string]string{}, dynWrapped: map[*Class][]dynWrapped{}, warned: map[string]bool{},
		preludeFS: preludeFS, parser: p, loaded: map[string]bool{}, dynEvery: dynEvery, dynOut: map[string]bool{}, respondOut: map[string]bool{}, fwdOut: map[*Class]bool{}, labels: map[string]string{}, erbSnippets: map[*parser.CallNode]*File{}}
	c.loadPath = opts.loadPath
	c.infer, c.round, c.inferDone = opts.infer, opts.round, opts.inferDone
	c.loadPreludeGo()
	c.loadPrelude(ctx, "prelude.rb")
	tick("prelude")
	for _, src := range sources {
		path := src.Path
		if path == "" {
			path = src.Name
		}
		path = realPath(path)
		if c.loaded[path] { // a file an earlier one require_relative'd: Ruby loads it once
			continue
		}
		c.loaded[path] = true
		uf, perr := parseFile(ctx, p, src.Name, src.Src, false)
		if perr != nil {
			return nil, perr
		}
		uf.path = path
		c.files = append(c.files, uf)
		c.userFiles = append(c.userFiles, uf)
		c.collect(ctx, uf)
	}
	for _, uf := range c.userFiles {
		c.collectERB(ctx, uf) // templates need every constant collected (decision 111)
	}
	tick("user")
	c.mainFile = c.userFiles[0]
	c.nameGo()
	c.link(ctx)
	c.guardConsts() // before any body is typed: defined?(X) on a guarded constant is String?
	tick("link")
	c.discoverIvars()
	if c.round {
		_ = catchCompileError(c.refineIvars)
		for _, m := range c.inferredMethods() {
			m.Ret = nil
		}
		for _, m := range c.inferredMethods() {
			_ = catchCompileError(func() { c.inferRet(m) })
		}
		c.collectUses()
		c.updateInference(opts.infer)
		return nil, errInferRound
	}
	c.refineIvars()
	if opts.skipTests {
		opts.skipped = c.skipFailingTests()
	}
	c.inferReturns()
	tick("infer")
	c.emitProgram()
	tick("emit")
	src := []byte(c.out.String())
	lazy := map[string]bool{}
	for _, k := range c.constList {
		if k.File.prelude {
			lazy[k.GoName] = true
		}
	}
	formatted, ferr := formatGo(src, lazy, c.emitNext, c.labels)
	tick("format")
	opts.warnings = c.Warnings // the tail's bodies warn too
	if errors.Is(ferr, errPruneIncomplete) {
		return nil, ferr
	}
	if ferr != nil {
		// Keep the raw output around for debugging.
		tmp := filepath.Join(os.TempDir(), "rb2go-bad-output.go")
		_ = os.WriteFile(tmp, slices.Concat(src, []byte("\n// ---- tail ----\n"), []byte(c.out.String())), 0o600)
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
func CompileWithWarnings(ctx context.Context, preludeFS fs.FS, mainName string, mainSrc []byte, loadPath ...string) ([]byte, []string, error) {
	return CompileFilesWithWarnings(ctx, preludeFS, []Source{{Name: filepath.Base(mainName), Src: mainSrc, Path: mainName}}, loadPath...)
}

// CompileFilesWithWarnings compiles several user files into one program, loaded in order (decision 84).
func CompileFilesWithWarnings(ctx context.Context, preludeFS fs.FS, sources []Source, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	opts := options{loadPath: loadPath}
	out, err := compile(ctx, preludeFS, sources, &opts)
	return out, opts.warnings, err
}

// CompileTestsSkipping is CompileFilesWithWarnings for a test suite that should run what compiles: each test_ method rb2go cannot compile becomes a skip carrying the error, and is listed.
// seed, when not nil, carries inferred parameter types between compiles of nearly the same sources.
func CompileTestsSkipping(ctx context.Context, preludeFS fs.FS, sources []Source, seed *Inference) ([]byte, []string, []SkippedTest, error) {
	if len(sources) == 0 {
		return nil, nil, nil, errors.New("no Ruby files to compile")
	}
	opts := options{skipTests: true, seed: seed}
	out, err := compile(ctx, preludeFS, sources, &opts)
	return out, opts.warnings, opts.skipped, err
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
	f, err := preludeFile(ctx, c.parser, name, src)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	c.files = append(c.files, f)
	c.collect(ctx, f)
	if len(c.mainStmts) > 0 {
		c.errorf(f, c.mainStmts[0], "prelude must not have top-level statements")
	}
}

// preludeFiles caches each prelude file's parse for the process: the prelude is constant, a File is never written after parseFile, and under wasm Prism runs interpreted, where parsing it costs 2.5 s of a 5.5 s compile.
var preludeFiles sync.Map // name → *File

func preludeFile(ctx context.Context, p *parser.Parser, name string, src []byte) (*File, error) {
	if f, ok := preludeFiles.Load(name); ok {
		return f.(*File), nil
	}
	f, err := parseFile(ctx, p, name, src, true)
	if err != nil {
		return nil, err
	}
	preludeFiles.Store(name, f)
	return f, nil
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

// userRequireRelative loads the file a user file's top-level
// `require_relative "x"` names (decision 130), as MRI does: by real path,
// once, its `__FILE__` that path. It returns nil when the file is already
// loaded.
func (c *Compiler) userRequireRelative(ctx context.Context, f *File, n *parser.CallNode) *File {
	args := callArgs(n)
	if len(args) != 1 {
		c.errorf(f, n, "require_relative needs a string literal")
	}
	str, ok := args[0].(*parser.StringNode)
	if !ok {
		c.errorf(f, n, "require_relative needs a string literal: rb2go loads files at compile time")
	}
	target := filepath.Join(filepath.Dir(f.path), filepath.FromSlash(str.Unescaped.Value))
	if filepath.Ext(target) != ".rb" {
		target += ".rb"
	}
	src, err := os.ReadFile(target) //nolint:gosec // the user's program names it
	if err != nil {
		c.errorf(f, n, "cannot load such file -- %s", strings.TrimSuffix(target, ".rb"))
	}
	return c.loadUserFile(ctx, target, src)
}

// userRequire loads the file a user file's top-level `require "x"` finds
// on the load path (-I, decision 131), as userRequireRelative does; ok is
// false when it names none, so the require is the prelude's to handle.
func (c *Compiler) userRequire(ctx context.Context, f *File, n *parser.CallNode) (*File, bool) {
	target := c.findRequire(f, n)
	if target == "" {
		return nil, false
	}
	src, err := os.ReadFile(target) //nolint:gosec // found on the user's -I path
	if err != nil {
		c.errorf(f, n, "cannot load such file -- %s", strings.TrimSuffix(target, ".rb"))
	}
	return c.loadUserFile(ctx, target, src), true
}

// findRequire is the file `require "x"` names on the load path, the first
// -I directory holding x.rb, as MRI searches $LOAD_PATH; "" when none does.
func (c *Compiler) findRequire(f *File, n *parser.CallNode) string {
	if f.prelude || len(c.loadPath) == 0 {
		return ""
	}
	args := callArgs(n)
	if len(args) != 1 {
		return ""
	}
	str, ok := args[0].(*parser.StringNode)
	if !ok {
		return ""
	}
	name := filepath.FromSlash(str.Unescaped.Value)
	if filepath.Ext(name) != ".rb" {
		name += ".rb"
	}
	for _, dir := range c.loadPath {
		target := filepath.Join(dir, name)
		info, err := os.Stat(target) //nolint:gosec // a -I directory the user gave, and a name their program requires
		if err == nil && !info.IsDir() {
			return target
		}
	}
	return ""
}

// loadUserFile parses and collects a required file, keyed by real path as
// MRI's loaded features are; nil when it is already loaded.
func (c *Compiler) loadUserFile(ctx context.Context, target string, src []byte) *File {
	path := realPath(target)
	if c.loaded[path] {
		return nil
	}
	c.loaded[path] = true
	uf, err := parseFile(ctx, c.parser, path, src, false)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	uf.path = path
	c.files = append(c.files, uf)
	c.userFiles = append(c.userFiles, uf)
	if c.required == nil {
		c.required = map[*File]bool{}
	}
	c.required[uf] = true
	c.collect(ctx, uf)
	return uf
}

// realPath is name absolute with symlinks resolved, as MRI keys loaded features; name itself when that fails.
func realPath(name string) string {
	p, err := filepath.Abs(name)
	if err != nil {
		return name
	}
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return r
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

// phaseTimer reports each phase's duration to stderr when RB2GO_TIMING is set: where a compile spends its time, natively or as wasm.
func phaseTimer() func(string) {
	if os.Getenv("RB2GO_TIMING") == "" {
		return func(string) {}
	}
	last := time.Now()
	return func(name string) {
		now := time.Now()
		fmt.Fprintf(os.Stderr, "rb2go: %-8s %6dms\n", name, now.Sub(last).Milliseconds())
		last = now
	}
}
