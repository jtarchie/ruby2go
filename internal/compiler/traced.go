package compiler

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/boot"
)

var (
	gemDir = regexp.MustCompile(`/gems/([^/]+?)-\d[^/]*/`)
	extDir = regexp.MustCompile(`/extensions/[^/]+/[^/]+/([^/]+?)-\d[^/]*/`)
)

// TracedEntry is a traced run's decision 197 entry for main, and the -I dirs its requires resolve from (decision 296).
func TracedEntry(l *boot.Loaded, main string) (src []byte, dirs []string) {
	var b bytes.Buffer
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			fmt.Fprintf(&b, "require %q\n", name)
		}
	}
	abs, err := filepath.Abs(main)
	if err == nil {
		main = abs
	}
	native := map[string]bool{} // a gem with a C extension (fiddle) cannot compile: its requires stay unresolved
	for _, f := range l.Features {
		if m := extDir.FindStringSubmatch(f); m != nil {
			native[m[1]] = true
		}
	}
	for _, f := range l.Features {
		if f == main {
			continue
		}
		if m := gemDir.FindStringSubmatch(f); m != nil && (preludeLibs[m[1]] != "" || mriLibs[m[1]]) {
			add(m[1]) // a gem the prelude is: the vendored copy's files are its
			continue
		} else if m != nil && native[m[1]] {
			continue
		}
		if dir, rel := under(f, l.StdDirs); dir != "" {
			name := strings.TrimSuffix(rel, filepath.Ext(rel))
			if first, _, _ := strings.Cut(name, "/"); preludeLibs[name] != "" || mriLibs[first] && first != "enc" {
				add(name) // MRI internals (encodings, portable Ruby's helpers) have no rb2go meaning
			}
			continue
		}
		dir, rel := under(f, l.LoadPath)
		if dir == "" || !strings.HasSuffix(rel, ".rb") {
			continue // require_relative from a compiled file reaches it again; a C extension cannot compile
		}
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
		add(strings.TrimSuffix(rel, ".rb"))
	}
	return b.Bytes(), dirs
}

// under is the longest of dirs holding f, and f relative to it.
func under(f string, dirs []string) (dir, rel string) {
	for _, d := range dirs {
		if r, ok := strings.CutPrefix(f, strings.TrimSuffix(d, "/")+"/"); ok && len(d) > len(dir) {
			dir, rel = d, r
		}
	}
	return dir, rel
}

// CompileTrace runs main with args under MRI (rubyOpts first, then the -I dirs) and compiles what that run loaded as a
// traced compile of main (decision 296).
func CompileTrace(ctx context.Context, preludeFS, gemSigs fs.FS, main string, args, rubyOpts []string, loadPath ...string) ([]byte, []string, error) {
	opts := slices.Clone(rubyOpts)
	for _, d := range loadPath {
		opts = append(opts, "-I", d)
	}
	l, err := boot.Trace(ctx, main, args, opts)
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // boot's errors name the trace
	}
	src, dirs := TracedEntry(l, main)
	more, err := mainRequires(ctx, main, append(slices.Clone(loadPath), dirs...), src)
	if err != nil {
		return nil, nil, err
	}
	src = append(src, more...)
	dir, err := os.MkdirTemp("", "rb2go-entry-")
	if err != nil {
		return nil, nil, fmt.Errorf("trace: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	entry := filepath.Join(dir, "entry.rb")
	err = os.WriteFile(entry, src, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("trace: %w", err)
	}
	srcs := []Source{{Name: "entry.rb", Src: src, Path: entry}}
	return CompileTraced(ctx, preludeFS, gemSigs, srcs, main, append(slices.Clone(loadPath), dirs...)...)
}

// mainRequires is a `require` for each literal require in main, outside its methods, of a file on dirs that the entry
// lacks: the trace saw only the paths that ran, and main's own requires are its static closure (brew.rb's
// `rescue` requiring utils/backtrace, decision 296).
func mainRequires(ctx context.Context, main string, dirs []string, entry []byte) ([]byte, error) {
	data, err := os.ReadFile(main) //nolint:gosec // the program's main file, named by the caller
	if err != nil {
		return nil, fmt.Errorf("main: %w", err)
	}
	p, err := sharedParser()
	if err != nil {
		return nil, fmt.Errorf("prism: %w", err)
	}
	f, err := parseFile(ctx, p, main, data, originUser)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		if n == nil {
			return
		}
		if _, ok := n.(*parser.DefNode); ok {
			return
		}
		if call, ok := n.(*parser.CallNode); ok && call.Name == "require" && call.Receiver == nil {
			if str, ok := firstArg(callArgs(call)).(*parser.StringNode); ok && len(callArgs(call)) == 1 {
				line := fmt.Sprintf("require %q\n", str.Unescaped.Value)
				if !bytes.Contains(entry, []byte(line)) && !bytes.Contains(b.Bytes(), []byte(line)) && onPath(str.Unescaped.Value, dirs) {
					b.WriteString(line)
				}
			}
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(f.Root)
	return b.Bytes(), nil
}

// onPath reports whether name.rb is a file in one of dirs.
func onPath(name string, dirs []string) bool {
	for _, d := range dirs {
		p := filepath.Join(d, name+".rb")
		fi, err := os.Stat(p) //nolint:gosec // a path the program names
		if err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}
