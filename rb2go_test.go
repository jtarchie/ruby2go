package rb2go

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
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

func run(t *testing.T, dir string, name string, args ...string) (string, int, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...) //nolint:gosec // test helper; args are ours
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
		err = nil
	}
	if err != nil {
		return stdout.String() + stderr.String(), -1, err
	}
	if code != 0 {
		return stdout.String(), code, errors.New(stderr.String())
	}
	return stdout.String(), 0, nil
}

func TestExamples(t *testing.T) {
	requireRuby4(t)
	dirs, err := filepath.Glob("examples/*/main.rb")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	for _, rb := range dirs {
		dir := filepath.Dir(rb)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			testExample(t, dir)
		})
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
			sameAsRuby(t, filepath.Dir(rb), filepath.Base(rb), goBuild(t, gen))
		})
	}
}

var directive = regexp.MustCompile(`(?m)^# (error|warning|skip): (.*)$`)

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
				_, warnings, err := Compile(context.Background(), "main.rb", f.Data)
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

func testExample(t *testing.T, dir string) {
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
	// 2. transpile, gofmt, vet, lint, build
	src, err := os.ReadFile(filepath.Join(dir, "main.rb")) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	gen := transpile(t, "main.rb", src)
	fmtOut, _, err := run(t, gen, "gofmt", "-l", "main.go")
	if err != nil || strings.TrimSpace(fmtOut) != "" {
		t.Fatalf("gofmt: %v %s", err, fmtOut)
	}
	vetOut, _, err := run(t, gen, "go", "vet", ".")
	if err != nil {
		t.Fatalf("go vet: %v\n%s", err, vetOut)
	}
	// The generated code must be a good citizen too: lint it with the
	// generated-code config.
	lintCfg, err := os.ReadFile(".golangci.generated.yml")
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(gen, ".golangci.yml"), lintCfg, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	lintOut, _, err := run(t, gen, "golangci-lint", "run", "--allow-parallel-runners", "./...")
	if err != nil {
		t.Fatalf("golangci-lint on generated code: %v\n%s", err, lintOut)
	}
	// 3. compare with MRI
	sameAsRuby(t, dir, "main.rb", goBuild(t, gen))
}

// transpile compiles src into a fresh Go module and returns its directory.
func transpile(t *testing.T, name string, src []byte) string {
	t.Helper()
	code, warnings, err := Compile(context.Background(), name, src)
	if err != nil {
		t.Fatalf("rb2go: %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}
	gen := t.TempDir()
	err = os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(gen, "go.mod"), []byte("module gen\n\ngo 1.24\n"), 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("generated Go: %s", filepath.Join(gen, "main.go"))
	return gen
}

func goBuild(t *testing.T, gen string) string {
	t.Helper()
	bin := filepath.Join(gen, "prog")
	out, _, err := run(t, gen, "go", "build", "-o", bin, ".")
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// sameAsRuby runs `ruby file` and bin in dir; stdout and exit code must match.
func sameAsRuby(t *testing.T, dir, file, bin string) {
	t.Helper()
	wantOut, wantCode, _ := run(t, dir, "ruby", file)
	gotOut, gotCode, _ := run(t, dir, bin)
	if wantOut != gotOut {
		t.Errorf("stdout differs\n--- ruby ---\n%s\n--- go ---\n%s", wantOut, gotOut)
	}
	if wantCode != gotCode {
		t.Errorf("exit code: ruby %d, go %d", wantCode, gotCode)
	}
}

var requireLine = regexp.MustCompile(`(?m)^require "([^"]+)"`)

// rbsLibraries turns an example's `require "net/http"` lines into the
// `-r net-http` flags rbs needs to see those libraries' signatures.
func rbsLibraries(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	matches := requireLine.FindAllStringSubmatch(string(src), -1)
	args := make([]string, 0, 2*len(matches))
	for _, m := range matches {
		args = append(args, "-r", strings.ReplaceAll(m[1], "/", "-"))
	}
	return args
}
