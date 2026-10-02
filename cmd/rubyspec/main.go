// Command rubyspec runs ruby/spec directories under rb2go, one minitest program each, so an example rb2go can't compile is skipped with its error instead of losing the directory.
package main

import (
	"cmp"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/jtarchie/ruby2go"
)

//go:embed mspec.rb
var mspec string

// rubyVersion is what version guards compare with.
// ponytail: MRI's version rb2go targets, by hand; read it from the prelude once it defines RUBY_VERSION.
const rubyVersion = "4.0.0"

// maxRewrites bounds the compile-skip loop for one program.
const maxRewrites = 400

type options struct {
	verbose bool
	timeout time.Duration
	keep    bool
}

func main() {
	spec := flag.String("spec", "", "ruby/spec checkout (default: cloned into the user cache, as scripts/rubyspec-coverage does)")
	jobs := flag.Int("j", max(1, runtime.NumCPU()/2), "programs compiled and run at once")
	reasons := flag.Int("reasons", 10, "list this many of the commonest unsupported reasons")
	var opts options
	flag.BoolVar(&opts.verbose, "v", false, "list every unsupported example and each program's failures")
	flag.DurationVar(&opts.timeout, "timeout", time.Minute, "per program run")
	flag.BoolVar(&opts.keep, "work", false, "keep each program's module (main.go) and print its path")
	flag.Parse()
	root, err := specRoot(*spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	targets, err := expand(root, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx := context.Background()
	p, err := parser.NewParser(ctx, parser.WithVersion(parser.SyntaxVersionLatest))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	results := make([]*result, len(targets))
	sem := make(chan struct{}, *jobs)
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = runTarget(ctx, p, root, t, opts)
			results[i].print(opts.verbose)
		})
	}
	wg.Wait()
	report(results, *reasons)
}

// specRoot is the given checkout, or a shallow clone in the user cache.
func specRoot(given string) (string, error) {
	if given != "" {
		return filepath.Abs(given) //nolint:wrapcheck // names the path
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cache dir: %w", err)
	}
	dir := filepath.Join(cache, "rb2go-rubyspec")
	_, err = os.Stat(dir)
	if err == nil {
		return dir, nil
	}
	cmd := exec.CommandContext(context.Background(), "git", "clone", "-q", "--depth", "1", "https://github.com/ruby/spec", dir) //nolint:gosec // dir is the user cache's
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("cloning ruby/spec: %w", err)
	}
	return dir, nil
}

// target is one program: a spec file, or a directory's *_spec.rb files.
type target struct {
	name  string   // as given, relative to the spec root
	files []string // relative to the spec root
}

// expand turns each argument into programs: a file is one, a directory of specs is one, and a directory of directories (core) is one per child.
func expand(root string, args []string) ([]target, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: rubyspec [-spec dir] [-j n] [-v] [-reasons n] path...  (core/integer, core/integer/abs_spec.rb, or core for each core/* directory)")
	}
	var out []target
	for _, a := range args {
		a = filepath.Clean(a)
		if strings.HasSuffix(a, ".rb") {
			out = append(out, target{name: a, files: []string{a}})
			continue
		}
		specs, _ := filepath.Glob(filepath.Join(root, a, "*_spec.rb"))
		if len(specs) > 0 {
			out = append(out, target{name: a, files: relAll(root, specs)})
			continue
		}
		subs, _ := filepath.Glob(filepath.Join(root, a, "*", "*_spec.rb"))
		if len(subs) == 0 {
			return nil, fmt.Errorf("rubyspec: no specs under %s", filepath.Join(root, a))
		}
		byDir := map[string][]string{}
		for _, s := range relAll(root, subs) {
			byDir[filepath.Dir(s)] = append(byDir[filepath.Dir(s)], s)
		}
		for _, d := range slices.Sorted(maps.Keys(byDir)) {
			out = append(out, target{name: d, files: byDir[d]})
		}
	}
	return out, nil
}

func relAll(root string, paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i], _ = filepath.Rel(root, p)
	}
	slices.Sort(out)
	return out
}

// skipped is an example (or a statement outside one) taken out, and why.
type skipped struct {
	where, reason string
}

type result struct {
	name                         string
	pass, fail, errored, skipped int
	unsupported                  []skipped // examples rewritten to skip
	compileSkips                 []skipped // examples the compiler skipped (CompileTestsSkipping)
	dropped                      []skipped // statements outside examples removed
	fatal                        string    // why the program produced no results
	output                       string    // the run's report, for -v
	took                         time.Duration
	compiles, builds             int           // rounds of each, for -v
	compileTime, buildTime       time.Duration // their totals
}

func runTarget(ctx context.Context, p *parser.Parser, root string, t target, opts options) *result {
	start := time.Now()
	r := &result{name: t.name}
	defer func() { r.took = time.Since(start) }()
	prog, err := load(ctx, p, root, t.files)
	if err != nil {
		r.fatal = err.Error()
		return r
	}
	bin, cleanup, err := prog.build(ctx, r, opts.keep)
	defer cleanup()
	if err != nil {
		r.fatal = err.Error()
		return r
	}
	r.run(ctx, root, bin, opts.timeout)
	return r
}

// testLine is one example's line in minitest's -v report.
var testLine = regexp.MustCompile(`#test_\d{4}_.* = [\d.]+ s = ([.FES])$`)

func (r *result) run(ctx context.Context, root, bin string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--seed", "1", "-v")
	cmd.Dir = root // specs name fixtures relative to the checkout
	out, err := cmd.CombinedOutput()
	r.output = string(out)
	for _, line := range strings.Split(r.output, "\n") {
		m := testLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch m[1] {
		case ".":
			r.pass++
		case "F":
			r.fail++
		case "E":
			r.errored++
		case "S":
			r.skipped++
		}
	}
	if !strings.Contains(r.output, " runs, ") {
		r.fatal = fmt.Sprintf("crashed (%v): %s", err, lastLines(r.output, 3))
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], " | ")
}

func (r *result) print(verbose bool) {
	if r.fatal != "" {
		fmt.Printf("%-28s FATAL %s\n", r.name, r.fatal)
	} else {
		total := r.pass + r.fail + r.errored + r.skipped
		fmt.Printf("%-28s %4d examples  %4d pass  %3d fail  %3d error  %4d skip (%d unsupported)  %.1fs\n",
			r.name, total, r.pass, r.fail, r.errored, r.skipped, len(r.unsupported)+len(r.compileSkips), r.took.Seconds())
	}
	if !verbose {
		return
	}
	fmt.Printf("    %d compiles %.1fs, %d builds %.1fs\n", r.compiles, r.compileTime.Seconds(), r.builds, r.buildTime.Seconds())
	for _, s := range r.dropped {
		fmt.Printf("    dropped %s: %s\n", s.where, s.reason)
	}
	for _, s := range slices.Concat(r.unsupported, r.compileSkips) {
		fmt.Printf("    unsupported %s: %s\n", s.where, s.reason)
	}
	if r.fail+r.errored > 0 {
		if i := strings.Index(r.output, "\n  1) "); i >= 0 {
			fmt.Println(r.output[i:])
		}
	}
}

// specOwner is a describe's name ending a message (`undefined method mock for Integer#div::fixnum`), which would split one reason per describe.
var specOwner = regexp.MustCompile(`( for )[^ ]*[#:.][^,]*$`)

// report totals every program and lists the commonest reasons examples were unsupported.
func report(results []*result, reasons int) {
	var pass, fail, errored, skip, unsup, fatal int
	why := map[string]int{}
	for _, r := range results {
		pass, fail, errored, skip, unsup = pass+r.pass, fail+r.fail, errored+r.errored, skip+r.skipped, unsup+len(r.unsupported)+len(r.compileSkips)
		if r.fatal != "" {
			fatal++
		}
		for _, s := range slices.Concat(r.unsupported, r.compileSkips, r.dropped) {
			why[specOwner.ReplaceAllString(s.reason, "${1}a spec")]++
		}
	}
	fmt.Printf("\ntotal: %d examples  %d pass  %d fail  %d error  %d skip (%d unsupported)  %d programs failed\n",
		pass+fail+errored+skip, pass, fail, errored, skip, unsup, fatal)
	keys := slices.SortedFunc(maps.Keys(why), func(a, b string) int { return cmp.Or(why[b]-why[a], strings.Compare(a, b)) })
	if len(keys) > reasons {
		keys = keys[:reasons]
	}
	if len(keys) > 0 {
		fmt.Println("\ncommonest unsupported reasons:")
	}
	for _, k := range keys {
		fmt.Printf("%6d  %s\n", why[k], k)
	}
}

// program is the files compiled together: mspec.rb, then each spec after the fixtures it requires.
type program struct {
	p      *parser.Parser
	root   string
	files  []*source
	byName map[string]*source
	shared map[string]sharedSpec
}

type source struct {
	name, text string
}

func load(ctx context.Context, p *parser.Parser, root string, specs []string) (*program, error) {
	prog := &program{p: p, root: root, byName: map[string]*source{}, shared: map[string]sharedSpec{}}
	prog.add(&source{name: "mspec.rb", text: mspec})
	for _, s := range specs {
		err := prog.load(ctx, s)
		if err != nil {
			return nil, err
		}
	}
	return prog, nil
}

func (prog *program) add(s *source) {
	prog.files = append(prog.files, s)
	prog.byName[s.name] = s
}

// load puts required fixtures first: rb2go has no require_relative in user code.
func (prog *program) load(ctx context.Context, name string) error {
	if _, ok := prog.byName[name]; ok {
		return nil
	}
	src, err := os.ReadFile(filepath.Join(prog.root, name)) //nolint:gosec // a spec, or a fixture it requires
	if err != nil {
		return err //nolint:wrapcheck // names the file
	}
	prog.byName[name] = nil // a require cycle stops here
	text, err := prog.rewrite(ctx, name, string(src), 0)
	if err != nil {
		return err
	}
	prog.add(&source{name: name, text: text})
	return nil
}

// build loops because rb2go stops at its first error; go build's -e reports all of them at once.
func (prog *program) build(ctx context.Context, r *result, keep bool) (string, func(), error) {
	dir, err := os.MkdirTemp("", "rubyspec-")
	if err != nil {
		return "", func() {}, fmt.Errorf("temp module: %w", err)
	}
	cleanup := func() {
		if keep {
			fmt.Fprintln(os.Stderr, "WORK="+dir)
			return
		}
		_ = os.RemoveAll(dir)
	}
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gen\n\ngo "+rb2go.GoVersion+"\n"), 0o600)
	if err != nil {
		return "", cleanup, fmt.Errorf("temp module: %w", err)
	}
	for range maxRewrites {
		code, err := prog.compile(ctx, r)
		if err != nil {
			return "", cleanup, err
		}
		if code == nil {
			continue // an example was skipped: compile again
		}
		err = os.WriteFile(filepath.Join(dir, "main.go"), code, 0o600)
		if err != nil {
			return "", cleanup, fmt.Errorf("temp module: %w", err)
		}
		cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-gcflags=-e", "-o", "prog", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
		start := time.Now()
		out, err := cmd.CombinedOutput()
		r.builds, r.buildTime = r.builds+1, r.buildTime+time.Since(start)
		if err == nil {
			return filepath.Join(dir, "prog"), cleanup, nil
		}
		err = prog.skipBuildErrors(ctx, r, string(out))
		if err != nil {
			return "", cleanup, err
		}
	}
	return "", cleanup, errors.New("too many rewrites")
}

// compile answers the Go program, or nil after skipping what the compile error names.
func (prog *program) compile(ctx context.Context, r *result) ([]byte, error) {
	files := make([]rb2go.File, len(prog.files))
	for i, f := range prog.files {
		files[i] = rb2go.File{Name: f.name, Src: []byte(f.text)}
	}
	start := time.Now()
	code, _, skips, err := rb2go.CompileTestsSkipping(ctx, files)
	r.compiles, r.compileTime = r.compiles+1, r.compileTime+time.Since(start)
	if err == nil {
		r.compileSkips = r.compileSkips[:0] // this compile's, not the last's
		for _, s := range skips {
			r.compileSkips = append(r.compileSkips, skipped{fmt.Sprintf("%s:%d %s", s.File, s.Line, s.Name), s.Reason})
		}
		return code, nil
	}
	msg := strings.TrimPrefix(err.Error(), "rb2go: ")
	name, line, reason, ok := location(msg)
	if !ok {
		return nil, fmt.Errorf("compile: %s", firstLine(msg))
	}
	return nil, prog.skipAt(ctx, r, name, line, reason)
}

// goError is a go build error at a Ruby line (the //line directives map them back).
var goError = regexp.MustCompile(`(?m)^(?:\./)?(\S+\.rb):(\d+)(?::\d+)?: (.*)$`)

func (prog *program) skipBuildErrors(ctx context.Context, r *result, out string) error {
	ms := goError.FindAllStringSubmatch(out, -1)
	if len(ms) == 0 {
		return fmt.Errorf("go build: %s", lastLines(out, 3))
	}
	for _, m := range ms {
		line, _ := strconv.Atoi(m[2])
		err := prog.skipAt(ctx, r, m[1], line, "go build: "+m[3])
		if err != nil {
			return err
		}
	}
	return nil
}

// location splits rb2go's "file:line: message".
func location(msg string) (string, int, string, bool) {
	first := firstLine(msg)
	parts := strings.SplitN(first, ":", 3)
	if len(parts) < 3 {
		return "", 0, "", false
	}
	line, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, "", false
	}
	return parts[0], line, strings.TrimSpace(parts[2]), true
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}
