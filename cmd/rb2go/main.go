// Command rb2go builds or runs a typed Ruby file as Go, like `go build` and `go run`.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"rb2go"
)

const usage = `usage: rb2go build [-o prog] [-race] [-gcflags flags] [-work] main.rb
       rb2go run [-race] [-gcflags flags] [-work] main.rb [args...]`

func main() {
	if len(os.Args) < 2 || os.Args[1] != "build" && os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
		fs.PrintDefaults()
	}
	var out *string
	if cmd == "build" {
		out = fs.String("o", "", "output binary (default: the file's name without .rb)")
	}
	opts := buildOpts{
		race:    fs.Bool("race", false, "build with the race detector"),
		gcflags: fs.String("gcflags", "", "passed to go build (e.g. -e for every error)"),
		work:    fs.Bool("work", false, "keep the temp module (main.go, go.mod) and print its path"),
	}
	_ = fs.Parse(os.Args[2:]) // ExitOnError
	if fs.NArg() < 1 || cmd == "build" && fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	file := fs.Arg(0)
	if cmd == "build" {
		bin := *out
		if bin == "" {
			bin = strings.TrimSuffix(filepath.Base(file), ".rb")
		}
		os.Exit(build(file, bin, opts))
	}
	os.Exit(run(file, fs.Args()[1:], opts))
}

type buildOpts struct {
	race    *bool
	gcflags *string
	work    *bool
}

func build(file, bin string, opts buildOpts) int {
	abs, err := filepath.Abs(bin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, err := compile(file, abs, opts)
	defer cleanup(dir, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(file string, args []string, opts buildOpts) int {
	dir, err := compile(file, "", opts)
	defer cleanup(dir, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Caller's cwd, as `ruby main.rb`; Ctrl-C reaches it via the shared process group.
	cmd := exec.CommandContext(context.Background(), filepath.Join(dir, "prog"), args...) //nolint:gosec // the binary we just built
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Outlive the child so the temp dir is removed; forward what didn't come via the process group (kill -TERM <rb2go>).
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	go func() {
		for sig := range sigs {
			_ = cmd.Process.Signal(sig)
		}
	}()
	err = cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()) // the shell's convention
		}
		return exit.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// compile builds into a fresh temp module each time: stdlib-only, so no network, and Go's content-keyed cache keeps rebuilds fast.
func compile(file, bin string, opts buildOpts) (string, error) {
	src, err := os.ReadFile(file) //nolint:gosec // the user's program
	if err != nil {
		return "", err //nolint:wrapcheck // the *PathError already names the file
	}
	code, warnings, err := rb2go.Compile(context.Background(), file, src)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if err != nil {
		return "", err //nolint:wrapcheck // compile errors carry file:line
	}
	dir, err := os.MkdirTemp("", "rb2go-")
	if err != nil {
		return "", fmt.Errorf("temp module: %w", err)
	}
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gen\n\ngo "+rb2go.GoVersion+"\n"), 0o600)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "main.go"), code, 0o600) //nolint:gosec // dir is our MkdirTemp
	}
	if err != nil {
		return dir, fmt.Errorf("temp module: %w", err)
	}
	if bin == "" {
		bin = filepath.Join(dir, "prog")
	}
	args := []string{"build", "-trimpath"}
	if *opts.race {
		args = append(args, "-race")
	}
	if *opts.gcflags != "" {
		args = append(args, "-gcflags="+*opts.gcflags)
	}
	cmd := exec.CommandContext(context.Background(), "go", append(args, "-o", bin, ".")...) //nolint:gosec // flags are the user's own
	cmd.Dir = dir
	// The user's go.work and GOFLAGS belong to their projects, not this module.
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if errors.Is(cmd.Err, exec.ErrNotFound) {
		return dir, errors.New("rb2go needs the go command on PATH to build")
	}
	err = cmd.Run()
	if err != nil {
		return dir, fmt.Errorf("go build: %w", err)
	}
	return dir, nil
}

func cleanup(dir string, opts buildOpts) {
	if dir == "" {
		return
	}
	if *opts.work {
		fmt.Fprintln(os.Stderr, "WORK="+dir)
		return
	}
	_ = os.RemoveAll(dir) //nolint:gosec // dir is our MkdirTemp
}
