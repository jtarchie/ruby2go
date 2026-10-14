package main

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// cliBinary builds this command once for the subprocess tests.
var cliBinary = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "rb2go-cli-test-") // not "rb2go-<x>": the tests count the CLI's own rb2go-* temp modules
	if err != nil {
		return "", err //nolint:wrapcheck // test helper
	}
	bin := filepath.Join(dir, "rb2go")
	out, err := exec.CommandContext(context.Background(), "go", "build", "-o", bin, ".").CombinedOutput() //nolint:gosec // bin is our MkdirTemp
	if err != nil {
		return "", errors.Join(err, errors.New(string(out)))
	}
	return bin, nil
})

func TestMain(m *testing.M) {
	code := m.Run()
	bin, err := cliBinary()
	if err == nil {
		_ = os.RemoveAll(filepath.Dir(bin))
	}
	os.Exit(code)
}

// moduleDirs lists the CLI's temp modules (rb2go-*) under tmp.
func moduleDirs(t *testing.T, tmp string) []string {
	t.Helper()
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "rb2go-") {
			out = append(out, e.Name())
		}
	}
	return out
}

// waitFor polls cond until it holds or the test's deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for !cond() {
		select {
		case <-t.Context().Done():
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(2 * time.Millisecond):
		}
	}
}

// exitCode is a subprocess's status as the shell reports it: 128+signal when signalled.
func exitCode(t *testing.T, err error) int {
	t.Helper()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case !errors.As(err, &exit):
		t.Fatal(err)
	}
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return exit.ExitCode()
}

// A signalled child is 128+signal for every subcommand, `rb2go test` included (its runProg used to return ExitCode()'s -1, so 255).
func TestRunProgSignalledChild(t *testing.T) {
	got := runProg(t.Context(), "/bin/sh", []string{"-c", "kill -TERM $$"})
	if got != 128+int(syscall.SIGTERM) {
		t.Errorf("exit %d, want %d", got, 128+int(syscall.SIGTERM))
	}
}

// TestInterruptCleansUp sends SIGINT to `rb2go run` once its temp module exists (so, during go build or after the program started) and wants the shell's 130 and the module gone: nothing handled signals before the program started, so Ctrl-C left a binary in $TMPDIR.
func TestInterruptCleansUp(t *testing.T) {
	bin, err := cliBinary()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		after func(t *testing.T, tmp string, stdout *bufio.Reader)
	}{
		{"temp_module_exists", func(t *testing.T, tmp string, _ *bufio.Reader) {
			waitFor(t, "the temp module", func() bool { return len(moduleDirs(t, tmp)) > 0 })
		}},
		{"program_running", func(t *testing.T, _ string, stdout *bufio.Reader) {
			line, err := stdout.ReadString('\n')
			if err != nil || line != "ready\n" {
				t.Fatalf("program said %q, %v", line, err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(t.TempDir(), "main.rb")
			err := os.WriteFile(src, []byte("$stdout.sync = true\nputs \"ready\"\nsleep 60\n"), 0o600)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), bin, "run", src) //nolint:gosec // built above
			cmd.Env = append(os.Environ(), "TMPDIR="+tmp)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			err = cmd.Start()
			if err != nil {
				t.Fatal(err)
			}
			tc.after(t, tmp, bufio.NewReader(stdout))
			err = cmd.Process.Signal(os.Interrupt)
			if err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			if got := exitCode(t, err); got != 130 {
				t.Errorf("exit %d, want 130", got)
			}
			if left := moduleDirs(t, tmp); len(left) > 0 {
				t.Errorf("temp modules left behind: %v", left)
			}
		})
	}
}
