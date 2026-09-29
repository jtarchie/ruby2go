package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// testOpts are `rb2go test`'s own flags; build flags are shared with build and run.
type testOpts struct {
	build    buildOpts
	verbose  bool
	run      string
	parallel int
	args     []string // after -args, for every test binary
}

// testBuild is one test file's compile result.
type testBuild struct {
	dir  string
	diag bytes.Buffer
	err  error
}

// testCmd is `rb2go test`, shaped like `go test`: each *_test.rb (or test_*.rb) file is its own
// program, built in parallel and run in its own directory, as `ruby x_test.rb` would be. Output is
// shown only for failures unless -v.
func testCmd(argv []string) int {
	opts, paths, err := parseTestFlags(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	files, err := testFiles(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "rb2go test: no test files (*_test.rb, test_*.rb) in", strings.Join(paths, " "))
		return 1
	}
	builds := buildTests(files, opts)
	status := 0
	for i, file := range files {
		if !runTest(file, &builds[i], opts) {
			status = 1
		}
		cleanup(builds[i].dir, opts.build)
	}
	return status
}

func parseTestFlags(argv []string) (testOpts, []string, error) {
	var opts testOpts
	if i := slices.Index(argv, "-args"); i >= 0 {
		argv, opts.args = argv[:i], argv[i+1:]
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
		fs.PrintDefaults()
	}
	fs.BoolVar(&opts.verbose, "v", false, "show every test's output (minitest -v)")
	fs.StringVar(&opts.run, "run", "", "run only tests whose name matches regexp (minitest -i /regexp/)")
	fs.IntVar(&opts.parallel, "p", runtime.GOMAXPROCS(0), "files to build in parallel")
	opts.build = buildOpts{
		race:    fs.Bool("race", false, "build with the race detector"),
		gcflags: fs.String("gcflags", "", "passed to go build"),
		work:    fs.Bool("work", false, "keep each temp module and print its path"),
	}
	err := fs.Parse(argv)
	if err != nil {
		return opts, nil, fmt.Errorf("rb2go test: %w", err)
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	return opts, paths, nil
}

// testFiles expands directories to their test files, recursively, as minitest's TEST_GLOB does (bar spec files); files named on the command line are taken as given.
func testFiles(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p) //nolint:gosec // the user's own paths
		if err != nil {
			return nil, err //nolint:wrapcheck // *PathError names the path
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		walk := func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() && path != p && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if !d.IsDir() && strings.HasSuffix(name, ".rb") && (strings.HasSuffix(name, "_test.rb") || strings.HasPrefix(name, "test_")) {
				out = append(out, path)
			}
			return nil
		}
		err = filepath.WalkDir(p, walk) //nolint:gosec // the user's own paths
		if err != nil {
			return nil, fmt.Errorf("rb2go test: %w", err)
		}
	}
	return out, nil
}

func buildTests(files []string, opts testOpts) []testBuild {
	builds := make([]testBuild, len(files))
	sem := make(chan struct{}, max(opts.parallel, 1))
	var wg sync.WaitGroup
	for i, file := range files {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			b := &builds[i]
			b.dir, b.err = compile(file, "", opts.build, &b.diag)
		})
	}
	wg.Wait()
	return builds
}

// runTest runs one built test file and prints go test's ok/FAIL line; it reports whether the file passed.
func runTest(file string, b *testBuild, opts testOpts) bool {
	if b.err != nil {
		_, _ = os.Stdout.Write(b.diag.Bytes())
		fmt.Printf("%v\nFAIL\t%s [build failed]\n", b.err, file)
		return false
	}
	args := slices.Clone(opts.args)
	if opts.verbose {
		_, _ = os.Stdout.Write(b.diag.Bytes()) // warnings
		args = append([]string{"-v"}, args...)
	}
	if opts.run != "" {
		args = append([]string{"-i", "/" + opts.run + "/"}, args...)
	}
	abs, err := filepath.Abs(filepath.Join(b.dir, "prog"))
	if err != nil {
		fmt.Printf("%v\nFAIL\t%s\n", err, file)
		return false
	}
	cmd := exec.CommandContext(context.Background(), abs, args...) //nolint:gosec // the binary we just built
	cmd.Dir = filepath.Dir(file)
	cmd.Stdin = os.Stdin
	var out bytes.Buffer
	var w io.Writer = &out
	if opts.verbose {
		w = os.Stdout
	}
	cmd.Stdout, cmd.Stderr = w, w
	start := time.Now()
	err = cmd.Run()
	took := time.Since(start).Seconds()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		fmt.Printf("%v\nFAIL\t%s\n", err, file)
		return false
	}
	if err != nil {
		_, _ = os.Stdout.Write(out.Bytes())
		fmt.Printf("FAIL\t%s\t%.3fs\n", file, took)
		return false
	}
	fmt.Printf("ok  \t%s\t%.3fs\n", file, took)
	return true
}
