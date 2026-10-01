package rb2go

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/txtar"
)

// gemCmd runs a Gemfile executable through bundler, so the versions pinned
// in Gemfile.lock are used regardless of what is on PATH.
func gemCmd(t *testing.T, exe string, args ...string) *exec.Cmd {
	t.Helper()
	return exec.CommandContext(t.Context(), "bundle", append([]string{"exec", exe}, args...)...) //nolint:gosec // test helper; args are ours
}

func requireRuby4(t *testing.T) {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "ruby", "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Fatalf("ruby is required: %v", err)
	}
	major, _ := strconv.Atoi(strings.SplitN(string(out), ".", 2)[0])
	if major < 4 {
		t.Fatalf("ruby >= 4.0 is required, found %s", out)
	}
	out, err = gemCmd(t, "rbs-inline", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("rbs-inline is required (run `bundle install`): %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	out, err = gemCmd(t, "rbs", "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("rbs is required (run `bundle install`): %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	_, err = exec.LookPath("golangci-lint")
	if err != nil {
		t.Fatalf("golangci-lint is required: %v", err)
	}
}

func run(t *testing.T, dir string, name string, args ...string) (string, error) {
	t.Helper()
	out, stderr, code, err := runIO(t, dir, progIO{}, name, args...)
	if err == nil && code != 0 {
		err = errors.New(stderr)
	}
	return out, err
}

// progIO is what a case feeds its program: `# args:` (space-separated), `# env: K=V` and `# stdin: "Go-quoted"` lines.
type progIO struct {
	env   []string
	stdin string
	trace bool // the trace oracle (decision 105): RB2GO_MT_TRACE on both runs, MRI with testdata/mt_trace.rb preloaded
}

func runIO(t *testing.T, dir string, pio progIO, name string, args ...string) (stdout, stderr string, code int, err error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...) //nolint:gosec // test helper; args are ours
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), pio.env...)
	cmd.Stdin = strings.NewReader(pio.stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.String(), errOut.String(), ee.ExitCode(), nil
	}
	if err != nil {
		return out.String() + errOut.String(), errOut.String(), -1, err
	}
	return out.String(), errOut.String(), 0, nil
}

func TestExamples(t *testing.T) {
	requireRuby4(t)
	dirs, err := filepath.Glob("examples/*/main.rb")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	// Every example's Go lands in one module, so vet and golangci-lint load
	// the standard library once for all of them instead of once per example.
	mod := filepath.Join(t.TempDir(), "gen")
	writeModule(t, mod)
	t.Run("each", func(t *testing.T) {
		for _, rb := range dirs {
			dir := filepath.Dir(rb)
			t.Run(filepath.Base(dir), func(t *testing.T) {
				t.Parallel()
				testExample(t, dir, filepath.Join(mod, filepath.Base(dir)))
			})
		}
	})
	if t.Failed() {
		return
	}
	lintGenerated(t, mod)
}

// lintGenerated vets and lints every generated example at once with the generated-code config.
func lintGenerated(t *testing.T, mod string) {
	t.Helper()
	vetOut, err := run(t, mod, "go", "vet", "./...")
	if err != nil {
		t.Fatalf("go vet: %v\n%s", err, vetOut)
	}
	lintCfg, err := os.ReadFile(".golangci.generated.yml")
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(mod, ".golangci.yml"), lintCfg, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	copyPreludeGo(t, mod)
	// --timeout=0: under the full suite's parallel builds this pass outlasts the config's 5m; go test's -timeout still bounds it
	lintOut, err := run(t, mod, "golangci-lint", "run", "--allow-parallel-runners", "--timeout=0", "./...")
	if err != nil {
		t.Fatalf("golangci-lint on generated code: %v\n%s", err, lintOut)
	}
}

// writeModule makes dir a Go module the generated programs build in.
func writeModule(t *testing.T, dir string) {
	t.Helper()
	err := os.MkdirAll(dir, 0o755) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gen\n\ngo "+GoVersion+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

// TestRun holds each testdata/run/*.rb to the same MRI comparison as the
// examples, without the rbs and lint gates: one go build per file.
func TestRun(t *testing.T) {
	requireRuby4(t)
	files, err := filepath.Glob("testdata/run/*.rb")
	if err != nil {
		t.Fatal(err)
	}
	for _, rb := range files {
		t.Run(strings.TrimSuffix(filepath.Base(rb), ".rb"), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(rb) //nolint:gosec // testdata path
			if err != nil {
				t.Fatal(err)
			}
			skipIfMarked(t, src)
			gen := transpile(t, filepath.Base(rb), src)
			// -l skips inlining, ~40% of compile CPU; TestExamples keeps the default build users get.
			sameAsRuby(t, filepath.Dir(rb), filepath.Base(rb), goBuild(t, gen, "-l"))
		})
	}
}

// TestMinitest runs each testdata/test/*_test.rb with a fixed seed. MRI must
// pass it, or the test itself is wrong; rb2go's output must then match MRI's
// apart from timings. One build per file for now: the files load together
// as one program (decision 84, `rb2go test testdata/test`), but that program
// takes Go ~30 minutes to build: Dyn wrappers grow as classes × names
// (decision 81).
func TestMinitest(t *testing.T) {
	requireRuby4(t)
	files, err := filepath.Glob("testdata/test/*_test.rb")
	if err != nil {
		t.Fatal(err)
	}
	for _, rb := range files {
		t.Run(strings.TrimSuffix(filepath.Base(rb), "_test.rb"), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(rb) //nolint:gosec // testdata path
			if err != nil {
				t.Fatal(err)
			}
			skipIfMarked(t, src)
			gen := transpile(t, filepath.Base(rb), src)
			sameAsRubyWith(t, filepath.Dir(rb), filepath.Base(rb), goBuild(t, gen, "-l"), []string{"--seed", "1"})
		})
	}
}

// TestMulti compiles each testdata/multi/<case>/*.rb, sorted, as one program
// (decision 84) and compares it with MRI loading the same files in one
// process, as rake's test loader does.
func TestMulti(t *testing.T) {
	requireRuby4(t)
	dirs, err := filepath.Glob("testdata/multi/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			paths, err := filepath.Glob(filepath.Join(dir, "*.rb"))
			if err != nil {
				t.Fatal(err)
			}
			files := make([]File, 0, len(paths))
			for _, p := range paths {
				src, rerr := os.ReadFile(p) //nolint:gosec // testdata path
				if rerr != nil {
					t.Fatal(rerr)
				}
				files = append(files, File{Name: p, Src: src})
			}
			code, warnings, err := CompileFiles(t.Context(), files)
			if err != nil {
				t.Fatalf("rb2go: %v", err)
			}
			for _, w := range warnings {
				t.Logf("warning: %s", w)
			}
			gen := t.TempDir()
			writeModule(t, gen)
			err = os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			bin := goBuild(t, gen, "-l")
			loader := append([]string{"-e", "ARGV.each { |f| require File.expand_path(f) }"}, paths...)
			wantOut, _, wantCode, _ := runIO(t, ".", progIO{}, "ruby", loader...)
			gotOut, _, gotCode, _ := runIO(t, ".", progIO{}, bin)
			if wantOut != gotOut || wantCode != gotCode {
				t.Errorf("ruby (exit %d):\n%s\n--- go (exit %d):\n%s", wantCode, wantOut, gotCode, gotOut)
			}
		})
	}
}

var directive = regexp.MustCompile(`(?m)^# (error|warning|skip|args|env|stdin|stderr): (.*)$`)

// directives returns the text of each `# kind: text` line in src.
func directives(src []byte, kind string) []string {
	var out []string
	for _, m := range directive.FindAllSubmatch(src, -1) {
		if string(m[1]) == kind {
			out = append(out, strings.TrimSpace(string(m[2])))
		}
	}
	return out
}

// skipIfMarked skips a case carrying `# skip: reason` (a known failure),
// unless RB2GO_RUN_SKIPPED is set.
func skipIfMarked(t *testing.T, src []byte) {
	t.Helper()
	reasons := directives(src, "skip")
	if len(reasons) > 0 && os.Getenv("RB2GO_RUN_SKIPPED") == "" {
		t.Skip("known failure: " + strings.Join(reasons, "; "))
	}
}

// TestTraceOracle checks decision 105's point: an assertion that passes on
// both sides but sees a different operand (here a process id) is reported.
func TestTraceOracle(t *testing.T) {
	requireRuby4(t)
	t.Parallel()
	dir := t.TempDir()
	src := []byte(`require "minitest/autorun"

class PidTest < Minitest::Test
  def test_pid = refute_equal(0, Process.pid)
end
`)
	err := os.WriteFile(filepath.Join(dir, "pid_test.rb"), src, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	bin := goBuild(t, transpile(t, "pid_test.rb", src), "-l")
	pio := progIO{trace: true}
	want := rubyOutput(t, dir, "pid_test.rb", src, pio, []string{"--seed", "1"})
	got := binOutput(t, dir, pio, bin, []string{"--seed", "1"})
	if want.Code != 0 || got.Code != 0 {
		t.Fatalf("both runs should pass: ruby %d, go %d\n%s%s", want.Code, got.Code, want.Stdout, got.Stdout)
	}
	d := traceDiff(want.Trace, got.Trace)
	if !strings.Contains(d, "PidTest#test_pid refute_equal 0 ") {
		t.Fatalf("the oracle should report the operand difference, got:\n%q\n--- traces ---\n%s---\n%s", d, want.Trace, got.Trace)
	}
	if traceDiff(want.Trace, want.Trace) != "" {
		t.Error("identical traces should not differ")
	}
}

// TestTypedAssertionsStayStatic checks decision 93's point: typed
// assertions with literal operators keep no run-time lookup by method name.
func TestTypedAssertionsStayStatic(t *testing.T) {
	t.Parallel()
	src := []byte(`require "minitest/autorun"

class T < Minitest::Test
  def test_x
    h = { a: 3 } #: Hash[Symbol, Integer]
    assert_equal 3, h[:a]
    assert_includes [1, 2], 2
    refute_empty "x"
    assert_operator 1, :<, 2
    assert_predicate 2, :even?
    assert_respond_to "x", :upcase
  end
end
`)
	out, _, err := compileSafe("main.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"func rbRespondsByName", "func rbDyn"} {
		if bytes.Contains(out, []byte(name)) {
			t.Errorf("generated code has %s", name)
		}
	}
}

// TestErrors compiles each file of testdata/errors/*.txtar as main.rb.
// Every `# error: text` must appear in the compile error (so compilation
// must fail), and every `# warning: text` in some warning; with no expected
// error, compilation must succeed.
func TestErrors(t *testing.T) {
	archives, err := filepath.Glob("testdata/errors/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range archives {
		ar, err := txtar.ParseFile(path)
		if err != nil {
			t.Fatal(err)
		}
		area := strings.TrimSuffix(filepath.Base(path), ".txtar")
		for _, f := range ar.Files {
			t.Run(area+"/"+strings.TrimSuffix(f.Name, ".rb"), func(t *testing.T) {
				t.Parallel()
				skipIfMarked(t, f.Data)
				wantErrs, wantWarns := directives(f.Data, "error"), directives(f.Data, "warning")
				if len(wantErrs)+len(wantWarns) == 0 {
					t.Fatal("case declares no `# error:` or `# warning:`")
				}
				_, warnings, err := compileSafe("main.rb", f.Data)
				switch {
				case len(wantErrs) > 0 && err == nil:
					t.Errorf("compiled; want error %q", wantErrs)
				case len(wantErrs) == 0 && err != nil:
					t.Errorf("unexpected error: %v", err)
				}
				for _, w := range wantErrs {
					if err != nil && !strings.Contains(err.Error(), w) {
						t.Errorf("error %q does not contain %q", err, w)
					}
				}
				for _, w := range wantWarns {
					if !slices.ContainsFunc(warnings, func(s string) bool { return strings.Contains(s, w) }) {
						t.Errorf("no warning contains %q; got %q", w, warnings)
					}
				}
			})
		}
	}
}

var rdocNoise = regexp.MustCompile(`(?m)^.*rdoc.*warning.*\n`)

func testExample(t *testing.T, dir, gen string) {
	tmp := t.TempDir()
	// 1. rbs-inline + rbs validate
	sig := filepath.Join(tmp, "sig")
	out, err := gemCmd(t, "rbs-inline", "--output="+sig, filepath.Join(dir, "main.rb")).CombinedOutput()
	if err != nil {
		t.Fatalf("rbs-inline failed: %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	args := append(rbsLibraries(t, filepath.Join(dir, "main.rb")), "-I", sig, "validate")
	out, err = gemCmd(t, "rbs", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("rbs validate failed: %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	// 2. transpile and build (vet and lint run once over all examples: lintGenerated)
	src, err := os.ReadFile(filepath.Join(dir, "main.rb")) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	writeGenerated(t, gen, "main.rb", src)
	fmtOut, err := run(t, gen, "gofmt", "-l", "main.go")
	if err != nil || strings.TrimSpace(fmtOut) != "" {
		t.Fatalf("gofmt: %v %s", err, fmtOut)
	}
	// 3. compare with MRI; examples keep inlining, the default build users get
	sameAsRuby(t, dir, "main.rb", goBuild(t, gen, ""))
}

// compileSafe is Compile with an internal compiler panic turned into an
// error, so one bad case fails its test instead of the whole binary.
func compileSafe(name string, src []byte) (code []byte, warnings []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compiler panic: %v\n%s", r, debug.Stack())
		}
	}()
	return Compile(context.Background(), name, src)
}

// transpile compiles src into a fresh Go module and returns its directory.
func transpile(t *testing.T, name string, src []byte) string {
	t.Helper()
	gen := t.TempDir()
	writeModule(t, gen)
	writeGenerated(t, gen, name, src)
	stablePreludeLines(t, filepath.Join(gen, "main.go"))
	return gen
}

var preludeLine = regexp.MustCompile(`(//line prelude/[^:\s]+):\d+`)

// stablePreludeLines sets every prelude //line to line 1, so the Go build cache keys on what the program reaches, not where in the prelude it sits (decision 88); the examples keep real lines for lint.
func stablePreludeLines(t *testing.T, path string) {
	t.Helper()
	code, err := os.ReadFile(path) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, preludeLine.ReplaceAll(code, []byte("${1}:1")), 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
}

// writeGenerated compiles src into dir/main.go.
func writeGenerated(t *testing.T, dir, name string, src []byte) {
	t.Helper()
	code, warnings, err := compileSafe(name, src)
	if err != nil {
		t.Fatalf("rb2go: %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
		if strings.HasPrefix(w, "prelude/") {
			t.Errorf("a prelude warning reaches users (mark the method # @dynamic if intended): %s", w)
		}
	}
	err = os.MkdirAll(dir, 0o755) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "main.go"), code, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("generated Go: %s", filepath.Join(dir, "main.go"))
}

// copyPreludeGo mirrors prelude/go/*.go into gen at the same relative path so golangci-lint can open the //line targets it remaps positions to (nolint matching needs the real file, not just the label).
func copyPreludeGo(t *testing.T, gen string) {
	t.Helper()
	files, err := filepath.Glob("prelude/go/*.go")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(gen, "prelude", "go")
	err = os.MkdirAll(dir, 0o755) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		data, rerr := os.ReadFile(f) //nolint:gosec // repo path
		if rerr != nil {
			t.Fatal(rerr)
		}
		err = os.WriteFile(filepath.Join(dir, filepath.Base(f)), data, 0o600) //nolint:gosec // under t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// goBuild builds gen's program; gcflags is extra compiler flags ("-l" skips inlining).
func goBuild(t *testing.T, gen string, gcflags string) string {
	t.Helper()
	bin := filepath.Join(gen, "prog")
	// -dwarf=false: debug info is a quarter of each cache entry and no test reads it (decision 88); a repeated -gcflags replaces the earlier one, so it is composed here
	gcflags = strings.TrimSpace(gcflags + " -dwarf=false")
	// -race: generated threads and queues must be race-free; -trimpath: the build cache hits across temp dirs.
	args := []string{"build", "-race", "-trimpath", "-gcflags=" + gcflags, "-o", bin, "."}
	out, err := run(t, gen, "go", args...)
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// mtTiming matches minitest's run timings: the `Finished in` line and -v's per-test seconds.
var mtTiming = regexp.MustCompile(`(?m)^Finished in .*$| = \d+\.\d\d s = `)

// mtTimings blanks minitest's timings, the only output that differs between two runs with one seed.
func mtTimings(s string) string { return mtTiming.ReplaceAllString(s, "<timing>") }

// sameAsRuby runs `ruby file` and bin in dir; stdout and exit code must match, and stderr too under `# stderr: match`.
func sameAsRuby(t *testing.T, dir, file, bin string) {
	t.Helper()
	sameAsRubyWith(t, dir, file, bin, nil)
}

// sameAsRubyWith is sameAsRuby with extra arguments after the file's `# args:`.
func sameAsRubyWith(t *testing.T, dir, file, bin string, extra []string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(dir, file)) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	pio := progIO{env: directives(src, "env")}
	args := []string{} //nolint:prealloc // field count unknown until split
	for _, a := range directives(src, "args") {
		args = append(args, strings.Fields(a)...)
	}
	args = append(args, extra...)
	for _, q := range directives(src, "stdin") {
		in, qerr := strconv.Unquote(q)
		if qerr != nil {
			t.Fatalf("# stdin: %s: %v", q, qerr)
		}
		pio.stdin += in
	}
	mt := bytes.Contains(src, []byte(`require "minitest`))
	pio.trace = mt
	want := rubyOutput(t, dir, file, src, pio, args)
	if extra != nil && want.Code != 0 {
		t.Fatalf("MRI fails %s, so the test itself is wrong:\n%s%s", file, want.Stdout, want.Stderr)
	}
	got := binOutput(t, dir, pio, bin, args)
	if mt {
		want.Stdout, got.Stdout = mtTimings(want.Stdout), mtTimings(got.Stdout)
	}
	if want.Stdout != got.Stdout {
		t.Errorf("stdout differs\n--- ruby ---\n%s\n--- go ---\n%s", want.Stdout, got.Stdout)
	}
	if want.Code != got.Code {
		t.Errorf("exit code: ruby %d, go %d", want.Code, got.Code)
	}
	if slices.Contains(directives(src, "stderr"), "match") && want.Stderr != got.Stderr {
		t.Errorf("stderr differs\n--- ruby ---\n%s\n--- go ---\n%s", want.Stderr, got.Stderr)
	}
	if d := traceDiff(want.Trace, got.Trace); d != "" {
		t.Errorf("assertion operands differ from MRI's (trace oracle, decision 105):\n%s", d)
	}
}

// binOutput runs the compiled program as rubyOutput runs MRI, collecting its trace when pio.trace is set.
func binOutput(t *testing.T, dir string, pio progIO, bin string, args []string) mriResult {
	t.Helper()
	var got mriResult
	var tracePath string
	if pio.trace {
		tracePath = filepath.Join(t.TempDir(), "trace")
		pio.env = append(slices.Clone(pio.env), "RB2GO_MT_TRACE="+tracePath)
	}
	got.Stdout, got.Stderr, got.Code, _ = runIO(t, dir, pio, bin, args...)
	if pio.trace {
		got.Trace = readTrace(t, tracePath)
	}
	return got
}

// readTrace is the trace file's content, empty when the program wrote none.
func readTrace(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // our temp path
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return string(data)
}

// traceAddr masks object addresses and ids, as minitest's own diff does (rbMtPPForDiff), and Ractor numbers: none agree across runs.
var traceAddr = regexp.MustCompile(`0x[0-9a-f]+|oid=\d+|Ractor:#\d+|id:\d+`)

// traceDiff describes the first line where two assertion traces differ, or is empty when they agree.
func traceDiff(want, got string) string {
	if want == got {
		return ""
	}
	w := strings.Split(strings.TrimSuffix(traceAddr.ReplaceAllString(want, "#"), "\n"), "\n")
	g := strings.Split(strings.TrimSuffix(traceAddr.ReplaceAllString(got, "#"), "\n"), "\n")
	for i := range max(len(w), len(g)) {
		wl, gl := "<end>", "<end>"
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d of %d (ruby) / %d (go):\n--- ruby ---\n%s\n--- go ---\n%s", i+1, len(w), len(g), wl, gl)
		}
	}
	return ""
}

// rubyDescription keys the MRI output cache: another Ruby may print differently.
var rubyDescription = sync.OnceValue(func() string {
	out, _ := exec.Command("ruby", "-e", "print RUBY_DESCRIPTION").Output()
	return string(out)
})

type mriResult struct {
	Stdout, Stderr string
	Trace          string // the assertion trace (decision 105), when asked for
	Code           int
}

// mriCached holds output as []byte (base64 in JSON): a string field would turn non-UTF-8 bytes into U+FFFD.
type mriCached struct {
	Stdout []byte `json:"stdout"`
	Stderr []byte `json:"stderr"`
	Trace  []byte `json:"trace,omitempty"`
	Code   int    `json:"code"`
}

// mtTracePreload is the MRI half of the trace oracle, read once: its source keys the cache too.
var mtTracePreload = sync.OnceValues(func() (string, []byte) {
	path, err := filepath.Abs("testdata/mt_trace.rb")
	if err != nil {
		panic(err)
	}
	src, err := os.ReadFile(path) //nolint:gosec // testdata path
	if err != nil {
		panic(err)
	}
	return path, src
})

// rubyOutput is `ruby file`'s result, cached by source (which holds the args/env/stdin directives), Ruby and TZ (RB2GO_NO_MRI_CACHE=1 skips the cache).
func rubyOutput(t *testing.T, dir, file string, src []byte, pio progIO, args []string) mriResult {
	t.Helper()
	return mriRun(t, dir, src, pio, append([]string{file}, args...))
}

// mriRun runs `ruby rubyArgs...` in dir, cached by src (every source it reads), Ruby's version, TZ and the arguments past the first.
func mriRun(t *testing.T, dir string, src []byte, pio progIO, rubyArgs []string) mriResult {
	t.Helper()
	key := "v4\x00" + rubyDescription() + "\x00" + os.Getenv("TZ") + "\x00"
	if args := rubyArgs[1:]; len(args) > 0 { // `# args:` are in src; TestMinitest adds more
		key += strings.Join(args, "\x00") + "\x00args\x00"
	}
	var tracePath string
	if pio.trace {
		preload, preloadSrc := mtTracePreload()
		key += "trace\x00" + string(preloadSrc) + "\x00"
		tracePath = filepath.Join(t.TempDir(), "trace")
		pio.env = append(slices.Clone(pio.env), "RB2GO_MT_TRACE="+tracePath)
		rubyArgs = append([]string{"-r", preload}, rubyArgs...)
	}
	sum := sha256.Sum256(slices.Concat([]byte(key), src))
	cacheDir, err := os.UserCacheDir()
	cached := filepath.Join(cacheDir, "rb2go-test", "mri", hex.EncodeToString(sum[:]))
	if err == nil && os.Getenv("RB2GO_NO_MRI_CACHE") == "" {
		data, rerr := os.ReadFile(cached) //nolint:gosec // our cache path
		var c mriCached
		if rerr == nil && json.Unmarshal(data, &c) == nil {
			return mriResult{Stdout: string(c.Stdout), Stderr: string(c.Stderr), Trace: string(c.Trace), Code: c.Code}
		}
	}
	var r mriResult
	r.Stdout, r.Stderr, r.Code, _ = runIO(t, dir, pio, "ruby", rubyArgs...)
	if pio.trace {
		r.Trace = readTrace(t, tracePath)
	}
	if err == nil && r.Code >= 0 {
		data, err := json.Marshal(mriCached{Stdout: []byte(r.Stdout), Stderr: []byte(r.Stderr), Trace: []byte(r.Trace), Code: r.Code})
		if err != nil {
			t.Fatal(err)
		}
		_ = os.MkdirAll(filepath.Dir(cached), 0o755) //nolint:gosec // cache dir
		_ = os.WriteFile(cached, data, 0o600)        //nolint:gosec // cache path
	}
	return r
}

var requireLine = regexp.MustCompile(`(?m)^require "([^"]+)"`)

// rbsLibraryNames covers requires whose gem name differs from its RBS stdlib signature directory, like `require "observer"` (defines Observable) shipping sigs under "observable".
var rbsLibraryNames = map[string]string{"observer": "observable", "minitest-autorun": "minitest"}

// rbsLibraries turns an example's `require "net/http"` lines into the `-r net-http` flags rbs needs to see those libraries' signatures. A library the rbs gem has no signatures for (`weakref`, `rexml`) has them vendored in sig/<name>.rbs (decision 110), loaded with -I instead.
func rbsLibraries(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	vendored, err := filepath.Abs("sig")
	if err != nil {
		t.Fatal(err)
	}
	matches := requireLine.FindAllStringSubmatch(string(src), -1)
	args := []string{"-I", vendored}
	for _, m := range matches {
		name := strings.ReplaceAll(m[1], "/", "-")
		if alias, ok := rbsLibraryNames[name]; ok {
			name = alias
		}
		_, serr := os.Stat(filepath.Join(vendored, name+".rbs")) //nolint:gosec // a require name from our own examples
		if serr == nil {
			continue
		}
		args = append(args, "-r", name)
	}
	return args
}
