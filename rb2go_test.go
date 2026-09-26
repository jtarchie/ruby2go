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
func gemCmd(exe string, args ...string) *exec.Cmd {
	return exec.Command("bundle", append([]string{"exec", exe}, args...)...)
}

func requireRuby4(t *testing.T) {
	t.Helper()
	out, err := exec.Command("ruby", "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Fatalf("ruby is required: %v", err)
	}
	major, _ := strconv.Atoi(strings.SplitN(string(out), ".", 2)[0])
	if major < 4 {
		t.Fatalf("ruby >= 4.0 is required, found %s", out)
	}
	if out, err := gemCmd("rbs-inline", "--help").CombinedOutput(); err != nil {
		t.Fatalf("rbs-inline is required (run `bundle install`): %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	if out, err := gemCmd("rbs", "--version").CombinedOutput(); err != nil {
		t.Fatalf("rbs is required (run `bundle install`): %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
}

func run(t *testing.T, dir string, name string, args ...string) (string, int, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
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
	if out, err := gemCmd("rbs-inline", "--output="+sig, filepath.Join(dir, "main.rb")).CombinedOutput(); err != nil {
		t.Fatalf("rbs-inline failed: %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	if out, err := gemCmd("rbs", "-I", sig, "validate").CombinedOutput(); err != nil {
		t.Fatalf("rbs validate failed: %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	// 2. transpile, gofmt, vet, build
	src, err := os.ReadFile(filepath.Join(dir, "main.rb"))
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
	if err := os.MkdirAll(gen, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gen, "main.go"), code, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gen, "go.mod"), []byte("module gen\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("generated Go: %s", filepath.Join(gen, "main.go"))
	if out, _, err := run(t, gen, "gofmt", "-l", "main.go"); err != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("gofmt: %v %s", err, out)
	}
	if out, _, err := run(t, gen, "go", "vet", "."); err != nil {
		t.Fatalf("go vet: %v\n%s", err, out)
	}
	bin := filepath.Join(tmp, "prog")
	if out, _, err := run(t, gen, "go", "build", "-o", bin, "."); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
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
