// Command rb2go builds or runs a typed Ruby file as Go, like `go build` and `go run`, or prints the Go it compiles to.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jtarchie/ruby2go"
)

const usage = `usage: rb2go build [-o prog] [-I dir]... [-race] [-gcflags flags] [-work] main.rb
       rb2go run [-I dir]... [-race] [-gcflags flags] [-work] main.rb [args...]
       rb2go gen [-I dir]... main.rb > main.go
       rb2go test [-I dir]... [-v] [-run regexp] [-p n] [-race] [-gcflags flags] [-work] [paths...] [-args args...]
       rb2go web [-addr 127.0.0.1:8080]
       rb2go web -static dir -assets https://host/path/`

func main() {
	if len(os.Args) < 2 || os.Args[1] != "build" && os.Args[1] != "run" && os.Args[1] != "test" && os.Args[1] != "gen" && os.Args[1] != "web" && os.Args[1] != "stubs" {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if os.Args[1] == "test" {
		os.Exit(testCmd(os.Args[2:]))
	}
	if os.Args[1] == "web" {
		os.Exit(webCmd(os.Args[2:]))
	}
	if os.Args[1] == "stubs" { // hidden: prelude/go/0_stubs.go for gopls (decision 154); TestPreludeGo regenerates it too
		os.Exit(stubs())
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
	fs.Func("I", "a directory `require` searches for your own files, as `ruby -I` (repeatable)", func(dir string) error {
		opts.loadPath = append(opts.loadPath, dir)
		return nil
	})
	_ = fs.Parse(os.Args[2:]) // ExitOnError
	if fs.NArg() < 1 || cmd != "run" && fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	file := fs.Arg(0)
	if cmd == "gen" {
		os.Exit(gen(file, opts.loadPath, os.Stdout))
	}
	if cmd == "build" {
		bin := *out
		if bin == "" {
			bin = strings.TrimSuffix(filepath.Base(file), ".rb")
		}
		os.Exit(build(file, bin, opts))
	}
	os.Exit(run(file, fs.Args()[1:], opts))
}

// gen writes the generated Go to out and warnings to stderr: the main.go that build -work keeps, without needing the go command.
func stubs() int {
	out, err := rb2go.PreludeStubs(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	err = os.WriteFile(rb2go.StubsPath, out, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func gen(file string, loadPath []string, out io.Writer) int {
	src, err := os.ReadFile(file) //nolint:gosec // the user's program
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	code, warnings, err := rb2go.CompileWithGems(context.Background(), file, src, loadPath...)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if err == nil {
		_, err = out.Write(code)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

type buildOpts struct {
	race     *bool
	gcflags  *string
	work     *bool
	loadPath []string // -I
}

func build(file, bin string, opts buildOpts) int {
	abs, err := filepath.Abs(bin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, err := compile(file, abs, opts, os.Stderr)
	defer cleanup(dir, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(file string, args []string, opts buildOpts) int {
	dir, err := compile(file, "", opts, os.Stderr)
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
		return exitStatus(exit)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// exitStatus is the child's exit code, or 128+signal for a signaled child (the shell's convention).
func exitStatus(exit *exec.ExitError) int {
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return exit.ExitCode()
}

// compile builds into a fresh temp module each time: stdlib-only, so no network, and Go's content-keyed cache keeps rebuilds fast. Warnings and go build's output go to diag.
func compile(file, bin string, opts buildOpts, diag io.Writer) (string, error) {
	src, err := os.ReadFile(file) //nolint:gosec // the user's program
	if err != nil {
		return "", err //nolint:wrapcheck // the *PathError already names the file
	}
	code, warnings, err := rb2go.CompileWithGems(context.Background(), file, src, opts.loadPath...)
	return buildModule(context.Background(), code, warnings, err, bin, opts, diag)
}

// compileFiles builds several files as one program, as Ruby loads them into one process (decision 84).
func compileFiles(files []string, bin string, opts buildOpts, diag io.Writer) (string, error) {
	srcs := make([]rb2go.File, 0, len(files))
	for _, f := range files {
		src, err := os.ReadFile(f) //nolint:gosec // the user's program
		if err != nil {
			return "", err //nolint:wrapcheck // the *PathError already names the file
		}
		srcs = append(srcs, rb2go.File{Name: f, Src: src})
	}
	code, warnings, err := rb2go.CompileFiles(context.Background(), srcs, opts.loadPath...)
	return buildModule(context.Background(), code, warnings, err, bin, opts, diag)
}

// buildModule writes the compiled Go into a fresh module and builds it.
func buildModule(ctx context.Context, code []byte, warnings []string, err error, bin string, opts buildOpts, diag io.Writer) (string, error) {
	for _, w := range warnings {
		_, _ = fmt.Fprintln(diag, "warning:", w)
	}
	if err != nil {
		return "", err
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
	cmd := exec.CommandContext(ctx, "go", append(args, "-o", bin, ".")...) //nolint:gosec // flags are the user's own
	cmd.Dir = dir
	// The user's go.work and GOFLAGS belong to their projects, not this module.
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	cmd.Stdout, cmd.Stderr = diag, diag
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
