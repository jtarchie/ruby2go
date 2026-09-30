package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func TestBuildAndRun(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.rb")
	err := os.WriteFile(src, []byte("puts \"hi #{1 + 2}\"\nexit 3\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f := false
	e := ""
	opts := buildOpts{race: &f, gcflags: &e, work: &f}

	bin := filepath.Join(dir, "prog")
	if code := build(src, bin, opts); code != 0 {
		t.Fatalf("build: exit %d", code)
	}
	out, err := exec.CommandContext(t.Context(), bin).Output() //nolint:gosec // built above
	if string(out) != "hi 3\n" {
		t.Errorf("stdout %q", out)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Errorf("want exit 3, got %v", err)
	}

	if code := run(src, nil, opts); code != 3 {
		t.Errorf("run: exit %d, want 3", code)
	}
	if code := run(filepath.Join(dir, "missing.rb"), nil, opts); code != 1 {
		t.Errorf("run missing file: exit %d, want 1", code)
	}
}

func TestSignalFlushes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "spin.rb")
	err := os.WriteFile(src, []byte("print \"x\"\n$stderr.puts \"ready\"\ni = 0\nwhile true\n  i += 1\nend\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f := false
	e := ""
	bin := filepath.Join(dir, "spin")
	if code := build(src, bin, buildOpts{race: &f, gcflags: &e, work: &f}); code != 0 {
		t.Fatalf("build: exit %d", code)
	}
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		var out strings.Builder
		cmd := exec.CommandContext(t.Context(), bin) //nolint:gosec // built above
		cmd.Stdout = &out
		stderr, err := cmd.StderrPipe()
		if err != nil {
			t.Fatal(err)
		}
		err = cmd.Start()
		if err != nil {
			t.Fatal(err)
		}
		ready := make([]byte, 6) // wait for the child's own readiness signal instead of a fixed sleep, which raced under load
		_, err = io.ReadFull(stderr, ready)
		if err != nil {
			t.Fatal(err)
		}
		_ = cmd.Process.Signal(sig)
		err = cmd.Wait()
		// MRI flushes buffered output, then dies by the signal.
		if out.String() != "x" {
			t.Errorf("%v: stdout %q, want buffered output flushed", sig, out.String())
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != sig {
			t.Errorf("%v: want death by signal, got %v", sig, err)
		}
	}
}

func TestTestCommand(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{ //nolint:gosec // Ruby sources, not credentials
		"pass_test.rb":     "require \"minitest/autorun\"\n\nclass PassTest < Minitest::Test\n  def test_ok = assert_equal(2, 1 + 1)\nend\n",
		"sub/fail_test.rb": "require \"minitest/autorun\"\n\nclass FailTest < Minitest::Test\n  def test_bad = assert_equal(3, 1 + 1)\n  def test_good = assert(true)\nend\n",
		"helper.rb":        "puts \"not a test file\"\n",
	}
	for name, src := range files {
		p := filepath.Join(dir, name)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(src), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	// one program, one run, one report, as `minitest dir` gives
	out, code := captureStdout(t, func() int { return testCmd([]string{dir, "--seed", "7"}) })
	if code != 1 {
		t.Errorf("exit %d, want 1 (a test fails)", code)
	}
	for _, want := range []string{
		"Run options: --seed 7\n",
		"FailTest#test_bad [" + filepath.Join(dir, "sub", "fail_test.rb") + ":4]:\nExpected: 3\n  Actual: 2\n",
		"3 runs, 3 assertions, 1 failures, 0 errors, 0 skips",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not a test file") {
		t.Errorf("helper.rb isn't a test file:\n%s", out)
	}

	// minitest's own flags pass through; rb2go's are taken out by name
	out, code = captureStdout(t, func() int {
		return testCmd([]string{"-n", "/good/", filepath.Join(dir, "sub", "fail_test.rb"), "-v", "--seed", "7"})
	})
	if code != 0 || !strings.Contains(out, "Run options: -n /good/ -v --seed 7\n") || !strings.Contains(out, "FailTest#test_good = ") {
		t.Errorf("-n /good/ -v: exit %d\n%s", code, out)
	}

	opts, rest, err := testBuildFlags([]string{"-v", "-race", "x_test.rb", "-gcflags", "-l", "--seed", "3", "-gcflags=-e"})
	if err != nil || !*opts.race || *opts.gcflags != "-e" || !slices.Equal(rest, []string{"-v", "x_test.rb", "--seed", "3"}) {
		t.Errorf("testBuildFlags: race=%v gcflags=%q rest=%q err=%v", *opts.race, *opts.gcflags, rest, err)
	}
}

// captureStdout runs f with os.Stdout sent to a file, returning what it wrote.
func captureStdout(t *testing.T, f func() int) (string, int) {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = tmp
	code := f()
	os.Stdout = orig
	out, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func TestGen(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.rb")
	err := os.WriteFile(src, []byte("puts \"hi\"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if code := gen(src, &out); code != 0 {
		t.Fatalf("gen: exit %d", code)
	}
	if !strings.Contains(out.String(), "\npackage main\n") {
		t.Errorf("not Go source: %.200q", out.String())
	}
	if code := gen(filepath.Join(dir, "missing.rb"), io.Discard); code != 1 {
		t.Errorf("gen missing file: exit %d, want 1", code)
	}
}
