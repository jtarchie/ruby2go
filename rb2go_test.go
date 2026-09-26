package rb2go

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
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
			testExample(t, dir)
		})
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
	// 2. transpile, gofmt, vet, build
	src, err := os.ReadFile(filepath.Join(dir, "main.rb")) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	code, warnings, err := Compile(context.Background(), "main.rb", src)
	if err != nil {
		t.Fatalf("rb2go: %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}
	gen := filepath.Join(tmp, "gen")
	err = os.MkdirAll(gen, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(gen, "go.mod"), []byte("module gen\n\ngo 1.24\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("generated Go: %s", filepath.Join(gen, "main.go"))
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
	lintOut, _, err := run(t, gen, "golangci-lint", "run", "./...")
	if err != nil {
		t.Fatalf("golangci-lint on generated code: %v\n%s", err, lintOut)
	}
	bin := filepath.Join(tmp, "prog")
	buildOut, _, err := run(t, gen, "go", "build", "-o", bin, ".")
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, buildOut)
	}
	// 3. compare with MRI
	wantOut, wantCode, _ := run(t, dir, "ruby", "main.rb")
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
