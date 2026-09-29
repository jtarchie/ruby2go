package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// testCmd is Ruby's `minitest` command: one program, one run, one report, because Ruby loads every test file into one process (decision 80).
func testCmd(argv []string) int {
	opts, rest, err := testBuildFlags(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	files, flags, err := expandTestArgs(rest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "rb2go test: no test files (test_*.rb, *_test.rb, spec_*.rb, *_spec.rb)")
		return 1
	}
	dir, err := compileFiles(files, "", opts, os.Stderr)
	defer cleanup(dir, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return runProg(filepath.Join(dir, "prog"), flags)
}

// testBuildFlags finds rb2go's flags anywhere: minitest's own (-v, -s N, -n /re/) never share their names.
func testBuildFlags(argv []string) (buildOpts, []string, error) {
	race, work, gcflags := false, false, ""
	opts := buildOpts{race: &race, gcflags: &gcflags, work: &work}
	var rest []string
	wantValue := false
	for _, a := range argv {
		switch {
		case wantValue:
			gcflags, wantValue = a, false
		case a == "-race":
			race = true
		case a == "-work":
			work = true
		case a == "-gcflags":
			wantValue = true
		case strings.HasPrefix(a, "-gcflags="):
			gcflags = strings.TrimPrefix(a, "-gcflags=")
		default:
			rest = append(rest, a)
		}
	}
	if wantValue {
		return opts, nil, errors.New("rb2go test: -gcflags needs a value")
	}
	return opts, rest, nil
}

// expandTestArgs splits args into test files and program flags, as
// minitest's PathExpander#process_args does. With no path, it is "test".
func expandTestArgs(args []string) ([]string, []string, error) {
	var files, excluded, flags []string
	sawPath := false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "-") && a != "-" && exists(a[1:]):
			sawPath = true
			ex, err := expandTestPath(a[1:])
			if err != nil {
				return nil, nil, err
			}
			excluded = append(excluded, ex...)
		case !strings.HasPrefix(a, "-") && exists(a):
			sawPath = true
			in, err := expandTestPath(a)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, in...)
		default:
			flags = append(flags, a)
		}
	}
	if !sawPath && exists("test") {
		in, err := expandTestPath("test")
		if err != nil {
			return nil, nil, err
		}
		files = in
	}
	var out []string
	for _, f := range files {
		if !slices.Contains(excluded, f) && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out, flags, nil
}

// expandTestPath is a file as given, or a directory's test files, recursively and sorted (Ruby's Dir[] sorts).
func expandTestPath(p string) ([]string, error) {
	info, err := os.Stat(p) //nolint:gosec // the user's own paths
	if err != nil {
		return nil, err //nolint:wrapcheck // *PathError names the path
	}
	if !info.IsDir() {
		return []string{p}, nil
	}
	var out []string
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		for _, pat := range []string{"test_*.rb", "*_test.rb", "spec_*.rb", "*_spec.rb"} {
			if ok, _ := filepath.Match(pat, d.Name()); ok {
				out = append(out, path)
				break
			}
		}
		return nil
	}
	err = filepath.WalkDir(p, walk) //nolint:gosec // the user's own paths
	if err != nil {
		return nil, fmt.Errorf("rb2go test: %w", err)
	}
	slices.Sort(out)
	return out, nil
}

func exists(p string) bool {
	_, err := os.Stat(p) //nolint:gosec // the user's own paths
	return err == nil
}

// runProg runs a built test program in the caller's directory, as `ruby` would, and returns its exit status.
func runProg(bin string, args []string) int {
	cmd := exec.CommandContext(context.Background(), bin, args...) //nolint:gosec // the binary we just built
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
