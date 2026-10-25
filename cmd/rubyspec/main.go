// Command rubyspec runs ruby/spec directories under rugaroo, one minitest program each, so an example rugaroo can't compile is skipped with its error instead of losing the directory.
package main

import (
	"cmp"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/jtarchie/rugaroo"
)

//go:embed mspec.rb
var mspec string

// rubyVersion is what version guards compare with.
// ponytail: MRI's version rugaroo targets, by hand; read it from the prelude once it defines RUBY_VERSION.
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
	flag.DurationVar(&opts.timeout, "timeout", 3*time.Second, "a test that prints nothing for this long is skipped as hung")
	flag.BoolVar(&opts.keep, "work", false, "keep each program's module (main.go) and print its path")
	printRoot := flag.Bool("print-root", false, "print the pinned checkout (fetching it if absent) and exit; scripts/rubyspec-coverage uses it")
	flag.Parse()
	err := opts.validate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	root, err := specRoot(*spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *printRoot {
		fmt.Println(root)
		return
	}
	targets, err := expand(root, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// SIGINT/SIGTERM cancel every build and run, so each program's deferred cleanup removes its temp module
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	p, err := parser.NewParser(ctx, parser.WithVersion(parser.SyntaxVersionLatest))
	if err != nil {
		stop()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1) //nolint:gocritic // stop() is called by hand
	}
	results := runTargets(targets, *jobs, func(t target) *result { return runTarget(ctx, p, root, t, opts) }, opts.verbose)
	report(results, *reasons)
	code := exitCode(results)
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "rubyspec: interrupted")
		code = 130
	}
	stop()
	os.Exit(code)
}

// validate rejects what would panic or never end the run: watch's ticker needs a positive interval.
func (o options) validate() error {
	if o.timeout < time.Millisecond {
		return fmt.Errorf("rubyspec: -timeout %v is too short (at least 1ms)", o.timeout)
	}
	return nil
}

// runTargets runs each target with at most jobs at once. A panic in one is that program's FATAL result, not the end of every other result.
func runTargets(targets []target, jobs int, run func(target) *result, verbose bool) []*result {
	results := make([]*result, len(targets))
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			defer func() {
				if p := recover(); p != nil {
					results[i] = &result{name: t.name, fatal: fmt.Sprintf("panic: %v", p)}
				}
				results[i].print(verbose)
			}()
			results[i] = run(t)
		})
	}
	wg.Wait()
	return results
}

// exitCode is 1 when any program produced no results, so a CI run over a broken toolchain is not a pass.
func exitCode(results []*result) int {
	if slices.ContainsFunc(results, func(r *result) bool { return r.fatal != "" }) {
		return 1
	}
	return 0
}

// specCommit pins the ruby/spec checkout: pass counts are only comparable between runs on the same specs.
const specCommit = "0816047460cc5c4ca23d09794b48e8638a7047cc"

// specRoot is the given checkout, or specCommit shallow-fetched into the user cache.
func specRoot(given string) (string, error) {
	if given != "" {
		return filepath.Abs(given) //nolint:wrapcheck // names the path
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cache dir: %w", err)
	}
	dir := filepath.Join(cache, "rugaroo-rubyspec")
	_, err = os.Stat(dir)
	if err == nil {
		have, herr := gitOutput(dir, "rev-parse", "HEAD")
		if herr == nil {
			if have != specCommit {
				fmt.Fprintf(os.Stderr, "rubyspec: %s is at %s, not the pinned %s; delete it to fetch the pin\n", dir, have, specCommit)
			}
			return dir, nil
		}
		fmt.Fprintf(os.Stderr, "rubyspec: %s is not a checkout (%v); fetching again\n", dir, herr)
		err = os.RemoveAll(dir)
		if err != nil {
			return "", fmt.Errorf("removing %s: %w", dir, err)
		}
	}
	// into a sibling and renamed, so an interrupted fetch never leaves a directory that looks complete
	tmp := dir + ".tmp"
	err = os.RemoveAll(tmp)
	if err == nil {
		err = os.MkdirAll(tmp, 0o750)
	}
	for _, args := range [][]string{{"init", "-q"}, {"fetch", "-q", "--depth", "1", "https://github.com/ruby/spec", specCommit}, {"checkout", "-q", "FETCH_HEAD"}} {
		if err != nil {
			break
		}
		cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // constant arguments
		cmd.Dir = tmp
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		err = cmd.Run()
	}
	if err == nil {
		err = os.Rename(tmp, dir)
	}
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("fetching ruby/spec %s: %w", specCommit, err)
	}
	return dir, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // callers pass constants
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// target is one program: a spec file, or a directory's *_spec.rb files.
type target struct {
	name  string   // as given, relative to the spec root
	files []string // relative to the spec root
}

// expand turns each argument into programs: a file is one, and a directory is one per directory under it (itself included) that directly holds *_spec.rb files, so core/array and core/array/pack are separate programs. A program named twice runs once.
func expand(root string, args []string) ([]target, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: rubyspec [-spec dir] [-j n] [-v] [-reasons n] path...  (core/integer, core/integer/abs_spec.rb, or core for every directory under core/ holding specs)")
	}
	var out []target
	seen := map[string]bool{}
	add := func(t target) {
		if !seen[t.name] {
			seen[t.name] = true
			out = append(out, t)
		}
	}
	for _, a := range args {
		a = filepath.Clean(a)
		if strings.HasSuffix(a, ".rb") {
			add(target{name: a, files: []string{a}})
			continue
		}
		byDir := map[string][]string{}
		err := filepath.WalkDir(filepath.Join(root, a), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(p, "_spec.rb") {
				rel, rerr := filepath.Rel(root, p)
				if rerr != nil {
					return rerr //nolint:wrapcheck // names the path
				}
				byDir[filepath.Dir(rel)] = append(byDir[filepath.Dir(rel)], rel)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("rubyspec: %w", err)
		}
		if len(byDir) == 0 {
			return nil, fmt.Errorf("rubyspec: no specs under %s", filepath.Join(root, a))
		}
		for _, d := range slices.Sorted(maps.Keys(byDir)) {
			slices.Sort(byDir[d])
			add(target{name: d, files: byDir[d]})
		}
	}
	return out, nil
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
	crashed                      string    // the last run's crash or hang
	why                          []string  // each skipped example's rugaroo reason, from the run
	excluded                     []string  // why each test a crash or hang took out did
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
	r.dropped = append(r.dropped, prog.unloadable...)
	for range maxCrashes {
		crash, err := r.attempt(func() (string, func(), error) { return prog.build(ctx, r, opts.keep) }, func(bin string) *crashSite {
			return r.runAll(func(excluded []string) *crashSite { return r.run(ctx, root, bin, opts.timeout, excluded) })
		})
		if err != nil {
			r.fatal = err.Error()
			return r
		}
		if crash == nil {
			return r
		}
		if crash.file == "" { // before any test, or nowhere a spec names
			r.fatal = r.crashed
			return r
		}
		// no test name to exclude: skip the example its stack was in, and build again
		err = prog.skipAt(ctx, r, crash.file, crash.line, crash.reason, nil)
		if err != nil {
			r.fatal = r.crashed
			return r
		}
	}
	r.fatal = r.crashed
	return r
}

// attempt defers the cleanup so a panic in the run (which runTargets recovers) cannot leak the temp module.
func (r *result) attempt(build func() (string, func(), error), run func(bin string) *crashSite) (*crashSite, error) {
	bin, cleanup, err := build()
	defer cleanup()
	if err != nil {
		return nil, err
	}
	return run(bin), nil
}

// runAll runs the program until it finishes, excluding each test that crashes or hangs it (a crash ends the whole run) by name, which needs no rebuild. Still crashing after maxCrashes*3 exclusions is a crash with no site, which runTarget reports as FATAL: it is not a finished run.
func (r *result) runAll(run func(excluded []string) *crashSite) *crashSite {
	var excluded []string
	for range maxCrashes * 3 {
		crash := run(excluded)
		if crash == nil || crash.test == "" {
			return crash
		}
		excluded = append(excluded, regexp.QuoteMeta(crash.test))
		r.unsupported = append(r.unsupported, skipped{crash.test, crash.reason})
		r.excluded = append(r.excluded, crash.reason)
	}
	r.crashed = fmt.Sprintf("gave up after %d crashing or hanging tests were excluded; last: %s", len(excluded), r.crashed)
	return &crashSite{reason: r.crashed}
}

// maxCrashes bounds the runs of one program that end in a crash or hang.
const maxCrashes = 10

// crashSite is where a crashed or hung run was: the first spec frame of its goroutine dump.
type crashSite struct {
	test   string // the test minitest -v named before running it
	file   string
	line   int
	reason string
}

// rbSkip is a skip rugaroo put in an example, in minitest -v's report: its reason follows "rugaroo: ".
var rbSkip = regexp.MustCompile(`(?m)\) Skipped:\n.*\n\s*rugaroo: (.*)$`)

// startedTest is minitest -v's "Class#test_name = " a test prints before its result.
var startedTest = regexp.MustCompile(`(?m)^(.*#test_\d{4}_.*?) = `)

// specFrame is a stack frame in a spec file, as the //line directives name it.
var specFrame = regexp.MustCompile(`(?m)^\s+(?:gen/)?(\S+_spec\.rb):(\d+)`)

// loadFrame is the next best, any ruby/spec file in an uncaught exception's report (RUGAROO_BACKTRACE): a fixture's top-level statement that raises while the program loads.
var loadFrame = regexp.MustCompile(`(?m)^(?:\s+from )?(?:gen/)?((?:core|library|language|command_line|optional|security|shared)/\S+\.rb|rbspec_mocks\.rb):(\d+):in `)

// testLine is one example's line in minitest's -v report.
var testLine = regexp.MustCompile(`#test_\d{4}_.* = [\d.]+ s = ([.FES])$`)

// idleWriter collects output and when it last came: minitest -v prints each test's name before running it, so silence means a test hangs.
type idleWriter struct {
	mu   sync.Mutex
	buf  strings.Builder
	last time.Time
}

func (w *idleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.last = time.Now()
	return w.buf.Write(p) //nolint:wrapcheck // a Builder never fails
}

func (w *idleWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *idleWriter) idle() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Since(w.last)
}

// watch cancels a run that has printed nothing for timeout.
func watch(ctx context.Context, cancel context.CancelFunc, w *idleWriter, timeout time.Duration) {
	tick := time.NewTicker(timeout / 10)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if w.idle() > timeout {
				cancel()
				return
			}
		}
	}
}

func (r *result) run(parent context.Context, root, bin string, timeout time.Duration, excluded []string) *crashSite {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	out := &idleWriter{last: time.Now()}
	go watch(ctx, cancel, out, timeout)
	args := []string{"--seed", "1", "-v"}
	if len(excluded) > 0 {
		args = append(args, "-e", "/^(?:"+strings.Join(excluded, "|")+")$/")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = root                                                                                                  // specs name fixtures relative to the checkout
	cmd.Env = append(os.Environ(), "RUGAROO_BACKTRACE=1", "SPEC_TEMP_DIR="+filepath.Join(filepath.Dir(bin), "tmp")) // in the module the runner removes, even after a crash
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGQUIT) }                                        // Go dumps every goroutine: the hung example's frame is in it
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	r.output = out.String()
	r.pass, r.fail, r.errored, r.skipped, r.crashed = 0, 0, 0, 0, ""
	r.why = r.why[:0]
	for _, m := range rbSkip.FindAllStringSubmatch(r.output, -1) {
		r.why = append(r.why, m[1])
	}
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
	if strings.Contains(r.output, " runs, ") {
		return nil
	}
	reason := "crashed: " + panicLine(r.output)
	if parent.Err() != nil { // a signal, not a hang
		r.crashed = "interrupted"
		return &crashSite{reason: r.crashed}
	}
	if ctx.Err() != nil {
		reason = fmt.Sprintf("hung: no output for %v", timeout)
	}
	r.crashed = fmt.Sprintf("%s (%v)", reason, err)
	site := &crashSite{reason: reason}
	if all := startedTest.FindAllStringIndex(r.output, -1); len(all) > 0 {
		at := all[len(all)-1][0]
		line, _, _ := strings.Cut(r.output[at:], "\n")
		if !testLine.MatchString(line) { // started, never finished
			site.test = startedTest.FindStringSubmatch(line)[1]
		}
	}
	m := specFrame.FindStringSubmatch(r.output)
	if m == nil {
		m = loadFrame.FindStringSubmatch(r.output)
	}
	if m != nil {
		site.file = m[1]
		site.line, _ = strconv.Atoi(m[2])
	}
	return site
}

// panicLine is a Go crash's cause, which its goroutine dump buries.
func panicLine(out string) string {
	for _, prefix := range []string{"fatal error: ", "panic: "} {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, prefix) {
				return l
			}
		}
	}
	return lastLines(out, 3)
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
			r.name, total+len(r.excluded), r.pass, r.fail, r.errored, r.skipped+len(r.excluded), len(r.why)+len(r.excluded), r.took.Seconds())
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

// report totals every program and lists the commonest reasons examples were unsupported: each skipped example once, as the run reported it.
func report(results []*result, reasons int) {
	var pass, fail, errored, skip, unsup, fatal int
	why := map[string]int{}
	for _, r := range results {
		pass, fail, errored, skip, unsup = pass+r.pass, fail+r.fail, errored+r.errored, skip+r.skipped+len(r.excluded), unsup+len(r.why)+len(r.excluded)
		if r.fatal != "" {
			fatal++
		}
		for _, reason := range slices.Concat(r.why, r.excluded) {
			why[specOwner.ReplaceAllString(reason, "${1}a spec")]++
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
	p          *parser.Parser
	root       string
	files      []*source
	byName     map[string]*source
	shared     map[string]sharedSpec
	unloadable []skipped          // files rewrite could not parse, left out
	seed       *rugaroo.Inference // parameter types from the last compile, which each cut barely changes
	mocks      []*mockClass       // mocks the rewrite made classes of, loaded as one more file
	mockN      int
	hoisted    []string // classes and modules examples define, moved to the top level
}

type source struct {
	name, text string
}

func load(ctx context.Context, p *parser.Parser, root string, specs []string) (*program, error) {
	prog := &program{p: p, root: root, byName: map[string]*source{}, shared: map[string]sharedSpec{}}
	text := mspec
	if os.Getenv("RUGAROO_INT_OVERFLOW") == "promote" { // a Bignum exists (decision 372), so mspec's helper does too
		text += "\ndef bignum_value(plus = 0) = 0x8000_0000_0000_0000 + plus\n"
	}
	prog.add(&source{name: "mspec.rb", text: text})
	for _, s := range specs {
		err := prog.load(ctx, s)
		if err != nil {
			return nil, err
		}
	}
	if len(prog.mocks) > 0 || len(prog.hoisted) > 0 {
		var b strings.Builder
		for _, h := range prog.hoisted {
			b.WriteString(h + "\n")
		}
		for _, m := range prog.mocks {
			b.WriteString(m.source())
		}
		prog.add(&source{name: "rbspec_mocks.rb", text: b.String()})
	}
	return prog, nil
}

func (prog *program) add(s *source) {
	prog.files = append(prog.files, s)
	prog.byName[s.name] = s
}

// load puts required fixtures first: rugaroo has no require_relative in user code.
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
	if err == nil {
		prog.add(&source{name: name, text: text})
	} else { // a file that won't parse is left out, not its directory
		prog.unloadable = append(prog.unloadable, skipped{name, err.Error()})
	}
	return nil
}

// build loops because rugaroo stops at its first error; go build's -e reports all of them at once.
func (prog *program) build(ctx context.Context, r *result, keep bool) (string, func(), error) {
	dir, err := os.MkdirTemp("", "rubyspec-")
	if err != nil {
		return "", func() {}, fmt.Errorf("temp module: %w", err)
	}
	// macOS's TMPDIR is under the /var -> /private/var symlink; specs compare tmp() against realpath and getcwd answers, which MRI's mspec temp dir (under the checkout) never meets.
	real, rerr := filepath.EvalSymlinks(dir)
	if rerr == nil {
		dir = real
	}
	cleanup := func() {
		if keep {
			fmt.Fprintln(os.Stderr, "WORK="+dir)
			return
		}
		_ = os.RemoveAll(dir)
	}
	err = rugaroo.WriteModule(dir)
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
		cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) } // go build tidies its own children on SIGINT
		cmd.WaitDelay = 10 * time.Second
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
	files := make([]rugaroo.File, len(prog.files))
	for i, f := range prog.files {
		files[i] = rugaroo.File{Name: f.name, Src: []byte(f.text)}
	}
	start := time.Now()
	if prog.seed == nil {
		prog.seed = rugaroo.NewInference()
	}
	code, _, skips, err := rugaroo.CompileTestsSkipping(ctx, files, prog.seed)
	r.compiles, r.compileTime = r.compiles+1, r.compileTime+time.Since(start)
	if err == nil {
		r.compileSkips = r.compileSkips[:0] // this compile's, not the last's
		for _, s := range skips {
			r.compileSkips = append(r.compileSkips, skipped{fmt.Sprintf("%s:%d %s", s.File, s.Line, s.Name), s.Reason})
		}
		return code, nil
	}
	msg := strings.TrimPrefix(err.Error(), "rugaroo: ")
	name, line, reason, ok := location(msg)
	if !ok {
		return nil, fmt.Errorf("compile: %s", firstLine(msg))
	}
	return nil, prog.skipAt(ctx, r, name, line, reason, nil)
}

// goError is a go build error at a Ruby line (the //line directives map them back).
var goError = regexp.MustCompile(`(?m)^(?:\./)?(\S+\.rb):(\d+)(?::\d+)?: (.*)$`)

func (prog *program) skipBuildErrors(ctx context.Context, r *result, out string) error {
	ms := goError.FindAllStringSubmatch(out, -1)
	if len(ms) == 0 {
		return fmt.Errorf("go build: %s", lastLines(out, 3))
	}
	seen := map[string]bool{}
	for _, m := range ms {
		line, _ := strconv.Atoi(m[2])
		err := prog.skipAt(ctx, r, m[1], line, "go build: "+m[3], seen)
		if err != nil {
			return err
		}
	}
	return nil
}

// location splits rugaroo's "file:line: message".
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
