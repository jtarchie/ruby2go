package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMain lets a test run main() as a child process (RUBYSPEC_TEST_MAIN=1), for flags and exit statuses.
func TestMain(m *testing.M) {
	if os.Getenv("RUBYSPEC_TEST_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func runMain(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...) //nolint:gosec // this test binary
	cmd.Env = append(os.Environ(), "RUBYSPEC_TEST_MAIN=1")
	out, err := cmd.Output()
	return string(out), err
}

func TestPrintRoot(t *testing.T) {
	root := t.TempDir()
	out, err := runMain(t, "-spec", root, "-print-root")
	if err != nil || strings.TrimSpace(out) != root {
		t.Errorf("-print-root = %q, %v; want %q", out, err, root)
	}
}

func TestCoverageScriptFailsOnEmptyCore(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "core/README")
	_, err := exec.LookPath("ruby")
	if err != nil {
		t.Fatal("ruby is required (the whole suite needs it)")
	}
	out, err := exec.CommandContext(t.Context(), "ruby", "../../scripts/rubyspec-coverage", "--spec", root).CombinedOutput() //nolint:gosec // our script
	if err == nil {
		t.Errorf("a ruby/spec checkout with no core specs must fail, not print NaN: %s", out)
	}
	if strings.Contains(string(out), "NaN") {
		t.Errorf("output has NaN: %s", out)
	}
}

func writeTree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, nil, 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func targetNames(ts []target) []string {
	out := make([]string, 0, len(ts))
	for _, tg := range ts {
		out = append(out, tg.name+" "+strings.Join(tg.files, ","))
	}
	return out
}

func TestExpandNested(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root,
		"core/array/a_spec.rb", "core/array/b_spec.rb", "core/array/shared/x.rb",
		"core/array/pack/p_spec.rb",
		"core/file/stat/s_spec.rb", "core/file/stat/deep/d_spec.rb",
		"core/file/f_spec.rb", "core/empty/fixtures/f.rb", "core/integer/i_spec.rb")
	got, err := expand(root, []string{"core"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"core/array core/array/a_spec.rb,core/array/b_spec.rb",
		"core/array/pack core/array/pack/p_spec.rb",
		"core/file core/file/f_spec.rb",
		"core/file/stat core/file/stat/s_spec.rb",
		"core/file/stat/deep core/file/stat/deep/d_spec.rb",
		"core/integer core/integer/i_spec.rb",
	}
	if !slices.Equal(targetNames(got), want) {
		t.Errorf("core expands to\n%q\nwant\n%q", targetNames(got), want)
	}
	// a directory of its own specs plus nested ones: separate targets
	got, err = expand(root, []string{"core/file"})
	if err != nil || len(got) != 3 {
		t.Errorf("core/file = %q, %v; want 3 targets", targetNames(got), err)
	}
	// a file stays one target, and the same program is not run twice
	got, err = expand(root, []string{"core/array/a_spec.rb", "core/array/a_spec.rb", "core/array/pack", "./core/array/pack/"})
	if err != nil || !slices.Equal(targetNames(got), []string{"core/array/a_spec.rb core/array/a_spec.rb", "core/array/pack core/array/pack/p_spec.rb"}) {
		t.Errorf("file + duplicates = %q, %v", targetNames(got), err)
	}
	// nothing to run is an error, not an empty pass
	for _, bad := range []string{"core/empty", "core/missing"} {
		_, err = expand(root, []string{bad})
		if err == nil {
			t.Errorf("expand(%s) found nothing and returned no error", bad)
		}
	}
}

func TestRunTargetsRecoversPanic(t *testing.T) {
	targets := []target{{name: "core/a"}, {name: "core/boom"}, {name: "core/c"}}
	results := runTargets(targets, 2, func(tg target) *result {
		if tg.name == "core/boom" {
			panic("kaboom")
		}
		return &result{name: tg.name, pass: 1}
	}, false)
	if len(results) != 3 || results[0].pass != 1 || results[2].pass != 1 {
		t.Fatalf("the other programs' results were lost: %+v", results)
	}
	if r := results[1]; r.name != "core/boom" || !strings.Contains(r.fatal, "kaboom") {
		t.Errorf("panicking target = %+v, want a FATAL result naming the panic", r)
	}
	if exitCode(results) != 1 {
		t.Error("a fatal program must exit non-zero")
	}
}

func TestRunAllGivesUp(t *testing.T) {
	// a program that crashes on a new test every run must not read as finished
	n := 0
	r := &result{}
	crash := r.runAll(func(excluded []string) *crashSite {
		n++
		r.crashed = "crashed: boom"
		return &crashSite{test: "T#test_" + strings.Repeat("x", n), reason: "crashed: boom"}
	})
	if crash == nil || crash.file != "" || !strings.Contains(r.crashed, "gave up") {
		t.Fatalf("runAll after %d crashing runs = %+v, crashed %q; want a file-less crash site that runTarget turns into FATAL", n, crash, r.crashed)
	}
	// a run that finishes is nil
	r = &result{}
	if got := r.runAll(func([]string) *crashSite { return nil }); got != nil {
		t.Errorf("finished run = %+v, want nil", got)
	}
}

func TestRunTargetCleansUp(t *testing.T) {
	// an error after the module exists, and a panic in the run, both remove it
	for _, how := range []string{"error", "panic"} {
		cleaned := 0
		r := &result{}
		func() {
			defer func() { _ = recover() }()
			_, _ = r.attempt(func() (string, func(), error) {
				if how == "error" {
					return "", func() { cleaned++ }, errors.New("go build failed")
				}
				return "bin", func() { cleaned++ }, nil
			}, func(string) *crashSite {
				if how == "panic" {
					panic("kaboom")
				}
				return nil
			})
		}()
		if cleaned != 1 {
			t.Errorf("%s: cleanup ran %d times, want 1", how, cleaned)
		}
	}
}

func TestOptionsValidate(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second, time.Nanosecond} {
		err := options{timeout: d}.validate()
		if err == nil {
			t.Errorf("timeout %v accepted; the watcher's ticker needs a positive interval", d)
		}
	}
	err := options{timeout: 3 * time.Second}.validate()
	if err != nil {
		t.Errorf("timeout 3s rejected: %v", err)
	}
}

func TestIdleWriterString(t *testing.T) {
	// the run reads the output after Wait gave up on the pipe (WaitDelay); the copier may still write
	w := &idleWriter{last: time.Now()}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			_, _ = w.Write([]byte("x"))
		}
	}()
	for range 1000 {
		_ = w.String()
	}
	<-done
	if got := len(w.String()); got != 1000 {
		t.Errorf("collected %d bytes, want 1000", got)
	}
}

func TestExitCode(t *testing.T) {
	if got := exitCode([]*result{{name: "a", fail: 3, errored: 2}}); got != 0 {
		t.Errorf("failing examples are reported, not fatal: exit %d", got)
	}
	if got := exitCode([]*result{{name: "a"}, {name: "b", fatal: "go build: x"}}); got != 1 {
		t.Errorf("one fatal program: exit %d, want 1", got)
	}
}

// TestMergeAndFailedFiles: a directory whose one program failed is run file by file; the files' results add up, and
// each file that still produced nothing counts as a failed program and fails the run.
func TestMergeAndFailedFiles(t *testing.T) {
	dir := &result{name: "language"}
	dir.merge(&result{pass: 3, fail: 1, errored: 2, skipped: 4, why: []string{"a"}, excluded: []string{"b"}, compiles: 1, builds: 1})
	dir.merge(&result{pass: 5, skipped: 1, why: []string{"c"}})
	dir.failedFiles = []string{"language/super_spec.rb: build failed"}
	if dir.pass != 8 || dir.fail != 1 || dir.errored != 2 || dir.skipped != 5 || len(dir.why) != 2 || len(dir.excluded) != 1 || dir.compiles != 1 {
		t.Errorf("merged = %+v", dir)
	}
	if exitCode([]*result{dir}) != 1 {
		t.Error("a run with a failed file must exit 1")
	}
	if exitCode([]*result{{name: "ok", pass: 1}}) != 0 {
		t.Error("a clean run must exit 0")
	}
	out := captureStdout(t, func() { report([]*result{dir}, 0) })
	if !strings.Contains(out, "total: 17 examples  8 pass  1 fail  2 error  6 skip (3 unsupported)  1 programs failed") {
		t.Errorf("report = %q", out)
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

// Three files in a row failing for one reason stop the isolation: the rest are listed with that reason, not compiled.
func TestIsolationStopsOnRepeatedFatal(t *testing.T) {
	calls := 0
	run := func(f string) *result {
		calls++
		if f == "d/ok_spec.rb" {
			return &result{name: f, pass: 1}
		}
		return &result{name: f, fatal: "gem.rb:1: unsupported call\nmore"}
	}
	merged := isolate(target{name: "d", files: []string{"d/a_spec.rb", "d/b_spec.rb", "d/c_spec.rb", "d/ok_spec.rb", "d/e_spec.rb"}}, &result{name: "d"}, run)
	if calls != 3 || len(merged.failedFiles) != 5 || merged.pass != 0 {
		t.Errorf("calls = %d, failed = %v, pass = %d; want 3 compiles, 5 failed files (two assumed)", calls, merged.failedFiles, merged.pass)
	}
	if merged.failedFiles[4] != "d/e_spec.rb: gem.rb:1: unsupported call" {
		t.Errorf("assumed file = %q", merged.failedFiles[4])
	}
}

// A thread's exception report between a test's name and its result still counts the result (library/drb).
func TestTestLineInterleaved(t *testing.T) {
	out := "A#test_0001_x = 0.00 s = .\n" +
		"B#test_0001_y = #<Thread> terminated with exception (report_on_exception is true): boom (TypeError)\n0.00 s = E\n" +
		"Finished in 0.004392s, 455.3604 runs/s, 0.0000 assertions/s.\n"
	var got []string
	for _, line := range strings.Split(out, "\n") {
		if m := testLine.FindStringSubmatch(line); m != nil {
			got = append(got, m[1])
		}
	}
	if !slices.Equal(got, []string{".", "E"}) {
		t.Fatalf("results %q, want [. E]", got)
	}
}
