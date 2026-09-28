package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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
	err := os.WriteFile(src, []byte("print \"x\"\ni = 0\nwhile true\n  i += 1\nend\n"), 0o600)
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
		err = cmd.Start()
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second) // ponytail: a startup race under load; a readiness signal needs stderr or File support
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
