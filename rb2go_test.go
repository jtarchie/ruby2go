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
	lintOut, err := run(t, mod, "golangci-lint", "run", "--allow-parallel-runners", "./...")
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
			sameAsRuby(t, filepath.Dir(rb), filepath.Base(rb), goBuild(t, gen, "-gcflags=-l"))
		})
	}
}

// TestMinitest runs each testdata/test/*_test.rb (minitest) with a fixed seed. MRI must pass it,
// or the test itself is wrong; rb2go's output must then match MRI's apart from timings. Like
// TestRun, it skips rbs and lint: one go build per file.
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
			sameAsRubyWith(t, filepath.Dir(rb), filepath.Base(rb), goBuild(t, gen, "-gcflags=-l"), []string{"--seed", "1"})
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
	sameAsRuby(t, dir, "main.rb", goBuild(t, gen))
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
	return gen
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

func goBuild(t *testing.T, gen string, flags ...string) string {
	t.Helper()
	bin := filepath.Join(gen, "prog")
	// -race: generated threads and queues must be race-free; -trimpath: the build cache hits across temp dirs.
	args := append(append([]string{"build", "-race", "-trimpath"}, flags...), "-o", bin, ".")
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
	want := rubyOutput(t, dir, file, src, pio, args)
	if extra != nil && want.Code != 0 {
		t.Fatalf("MRI fails %s, so the test itself is wrong:\n%s%s", file, want.Stdout, want.Stderr)
	}
	var got mriResult
	got.Stdout, got.Stderr, got.Code, _ = runIO(t, dir, pio, bin, args...)
	if bytes.Contains(src, []byte(`require "minitest`)) {
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
}

// rubyDescription keys the MRI output cache: another Ruby may print differently.
var rubyDescription = sync.OnceValue(func() string {
	out, _ := exec.Command("ruby", "-e", "print RUBY_DESCRIPTION").Output()
	return string(out)
})

type mriResult struct {
	Stdout, Stderr string
	Code           int
}

// mriCached holds output as []byte (base64 in JSON): a string field would turn non-UTF-8 bytes into U+FFFD.
type mriCached struct {
	Stdout []byte `json:"stdout"`
	Stderr []byte `json:"stderr"`
	Code   int    `json:"code"`
}

// rubyOutput is `ruby file`'s result, cached by source (which holds the args/env/stdin directives), Ruby and TZ (RB2GO_NO_MRI_CACHE=1 skips the cache).
func rubyOutput(t *testing.T, dir, file string, src []byte, pio progIO, args []string) mriResult {
	t.Helper()
	key := "v3\x00" + rubyDescription() + "\x00" + os.Getenv("TZ") + "\x00"
	if len(args) > 0 { // `# args:` are in src; TestMinitest adds more
		key += strings.Join(args, "\x00") + "\x00args\x00"
	}
	sum := sha256.Sum256(slices.Concat([]byte(key), src))
	cacheDir, err := os.UserCacheDir()
	cached := filepath.Join(cacheDir, "rb2go-test", "mri", hex.EncodeToString(sum[:]))
	if err == nil && os.Getenv("RB2GO_NO_MRI_CACHE") == "" {
		data, rerr := os.ReadFile(cached) //nolint:gosec // our cache path
		var c mriCached
		if rerr == nil && json.Unmarshal(data, &c) == nil {
			return mriResult{Stdout: string(c.Stdout), Stderr: string(c.Stderr), Code: c.Code}
		}
	}
	var r mriResult
	r.Stdout, r.Stderr, r.Code, _ = runIO(t, dir, pio, "ruby", append([]string{file}, args...)...)
	if err == nil && r.Code >= 0 {
		data, err := json.Marshal(mriCached{Stdout: []byte(r.Stdout), Stderr: []byte(r.Stderr), Code: r.Code})
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

// rbsLibraries turns an example's `require "net/http"` lines into the `-r net-http` flags rbs needs to see those libraries' signatures.
func rbsLibraries(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	matches := requireLine.FindAllStringSubmatch(string(src), -1)
	args := make([]string, 0, 2*len(matches))
	for _, m := range matches {
		name := strings.ReplaceAll(m[1], "/", "-")
		if alias, ok := rbsLibraryNames[name]; ok {
			name = alias
		}
		args = append(args, "-r", name)
	}
	return args
}
