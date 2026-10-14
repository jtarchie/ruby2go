package compiler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/boot"
)

// CompileWithBoot is CompileFilesWithWarnings for a program that requires a
// gem: it runs the program's load phase under MRI first (internal/boot,
// decision 161) and adds each autoload boot resolved as a source, so the
// compiler sees classes the source graph's literal requires do not name. A
// program whose entry is not on disk is compiled as usual. gemSigs holds
// vendored `.rbs` signatures (decision 160), keyed under `sig/gems/<gem>/`.
// gemDirs are the gems' lib dirs, searched after loadPath (the program's own
// -I): only files under them are gem code (decisions 169, 172, 173).
func CompileWithBoot(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, gemDirs []string, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	aug, autoloaded, m, err := bootSources(ctx, sources, append(slices.Clip(loadPath), gemDirs...))
	if err != nil {
		return nil, nil, err
	}
	opts := options{loadPath: loadPath, gemDirs: gemDirs, gemSigs: gemSigs, pruneGems: true, autoloaded: autoloaded, boot: m}
	out, err := compile(ctx, preludeFS, aug, &opts)
	return out, opts.warnings, err
}

// CompileWithGems compiles with the installed gems MRI finds for requires only a gem answers (decision 173); without such a require it never starts Ruby.
func CompileWithGems(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	if main, traced := tracedEntry(sources[0].Src); traced {
		return compileTraced(ctx, preludeFS, gemSigs, sources, main, loadPath)
	}
	names, err := gemRequires(ctx, sources, loadPath)
	var gemDirs []string
	if err == nil && len(names) > 0 {
		gemDirs, err = boot.FindGems(ctx, filepath.Dir(sourceName(sources[0])), names)
	}
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // the caller adds "rb2go: "
	}
	if len(gemDirs) == 0 {
		return CompileFilesWithWarnings(ctx, preludeFS, sources, loadPath...)
	}
	return CompileWithBoot(ctx, preludeFS, gemSigs, sources, gemDirs, loadPath...)
}

// bootSources runs boot on sources[0] and appends the files its resolved
// autoloads loaded, in first-seen order, that are not already sources, with each one's constant by real path.
func bootSources(ctx context.Context, sources []Source, loadPath []string) ([]Source, map[string]string, *boot.Manifest, error) {
	entry := sourceName(sources[0])
	_, statErr := os.Stat(entry)
	if statErr != nil {
		return sources, nil, nil, nil //nolint:nilerr // no file on disk (the playground): nothing to boot
	}
	m, err := boot.Capture(ctx, entry, loadPath...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("boot: %w", err)
	}
	consts := map[string]string{}
	seen := map[string]bool{}
	for _, s := range sources {
		seen[realPath(sourceName(s))] = true
	}
	out := sources
	for _, a := range m.Autoloads {
		if !a.Resolved || a.Loaded == "" || !strings.HasSuffix(a.Loaded, ".rb") || seen[realPath(a.Loaded)] {
			continue
		}
		seen[realPath(a.Loaded)] = true
		data, rerr := os.ReadFile(a.Loaded)
		if rerr != nil {
			return nil, nil, nil, fmt.Errorf("boot: %w", rerr)
		}
		out = append(out, Source{Name: filepath.Base(a.Loaded), Src: data, Path: a.Loaded})
		consts[realPath(a.Loaded)] = a.Const
	}
	return out, consts, m, nil
}

// neededAutoloads keeps the boot-resolved autoload files whose constant another program file names (decision 175); the rest are marked loaded, so never compiled.
func (c *Compiler) neededAutoloads(consts map[string]string) map[string]bool {
	if len(consts) == 0 {
		return nil
	}
	named := map[string]bool{}
	note := func(f *File) {
		anyNode(f.Root, func(n parser.Node) bool {
			switch n := n.(type) {
			case *parser.ConstantReadNode:
				named[n.Name] = true
			case *parser.ConstantPathNode:
				named[*n.Name] = true
			}
			return false
		})
	}
	for path, f := range c.parsed {
		if _, ok := consts[path]; !ok {
			note(f)
		}
	}
	needed := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for path, k := range consts {
			if !needed[path] && named[k] && c.parsed[path] != nil {
				needed[path], changed = true, true
				note(c.parsed[path])
			}
		}
	}
	for path := range consts {
		if !needed[path] {
			c.loaded[path] = true
		}
	}
	return needed
}

func sourceName(s Source) string {
	if s.Path != "" {
		return s.Path
	}
	return s.Name
}

// tracedEntry reports whether src is a decision 197 entry (`# rb2go_method_requires: runtime`) and the
// `# rb2go_main:` file it names, if any.
func tracedEntry(src []byte) (main string, traced bool) {
	for line := range strings.SplitSeq(string(src), "\n") {
		switch {
		case line == "# rb2go_method_requires: runtime":
			traced = true
		case strings.HasPrefix(line, "# rb2go_main: "):
			main = strings.TrimSpace(strings.TrimPrefix(line, "# rb2go_main: "))
		}
	}
	return main, traced
}

// compileTraced compiles a decision 197 entry: its -I dirs are gem dirs (boot, leniency and pruning, decisions
// 161/169/172), boot runs the entry's load list, and the main file it names is compiled after it, never booted.
func compileTraced(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, main string, gemDirs []string) ([]byte, []string, error) {
	aug, autoloaded, m, err := bootSources(ctx, sources, gemDirs)
	if err != nil {
		return nil, nil, err
	}
	if main != "" {
		data, rerr := os.ReadFile(main) //nolint:gosec // the entry names the program's main file
		if rerr != nil {
			return nil, nil, fmt.Errorf("rb2go_main: %w", rerr)
		}
		aug = append(aug, Source{Name: main, Src: data, Path: main})
	}
	poison := map[string]string{}
	var warnings []string
	for round := 0; ; round++ {
		opts := options{gemDirs: gemDirs, gemSigs: gemSigs, pruneGems: true, autoloaded: autoloaded, boot: m, mainLast: main != "", goPoison: poison}
		out, err := compile(ctx, preludeFS, aug, &opts)
		if err != nil || round == goPoisonRounds {
			return out, append(warnings, opts.warnings...), err
		}
		if dump := os.Getenv("RB2GO_GOPOISON_DUMP"); dump != "" { // debugging: the Go before its rejected gem methods become stubs
			_ = os.WriteFile(fmt.Sprintf("%s.%d", dump, round), out, 0o600) //nolint:gosec // a developer-chosen path, one per round
		}
		bad := goTypeErrors(out) // nil when it does not parse: go build reports that, with the generated file kept
		added := 0
		for fn, msg := range bad {
			if key, ok := opts.goFuncs[fn]; ok && poison[key] == "" {
				poison[key] = "Go rejects it: " + msg
				warnings = append(warnings, fmt.Sprintf("%s does not type-check in Go, so calling it raises NotImplementedError: %s (decision 205)", key, msg))
				added++
			}
		}
		if added == 0 {
			return out, append(warnings, opts.warnings...), err
		}
	}
}

// goPoisonRounds bounds the traced build's compiles: each poisons what Go rejected in the last (decision 205).
const goPoisonRounds = 4
