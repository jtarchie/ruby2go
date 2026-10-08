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
	aug, err := bootSources(ctx, sources, append(slices.Clip(loadPath), gemDirs...))
	if err != nil {
		return nil, nil, err
	}
	opts := options{loadPath: loadPath, gemDirs: gemDirs, gemSigs: gemSigs, pruneGems: true}
	out, err := compile(ctx, preludeFS, aug, &opts)
	return out, opts.warnings, err
}

// CompileWithGems compiles with the installed gems MRI finds for requires only a gem answers (decision 173); without such a require it never starts Ruby.
func CompileWithGems(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
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
// autoloads loaded, in first-seen order, that are not already sources.
func bootSources(ctx context.Context, sources []Source, loadPath []string) ([]Source, error) {
	entry := sourceName(sources[0])
	_, statErr := os.Stat(entry)
	if statErr != nil {
		return sources, nil //nolint:nilerr // no file on disk (the playground): nothing to boot
	}
	m, err := boot.Capture(ctx, entry, loadPath...)
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
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
			return nil, fmt.Errorf("boot: %w", rerr)
		}
		out = append(out, Source{Name: filepath.Base(a.Loaded), Src: data, Path: a.Loaded})
	}
	return out, nil
}

func sourceName(s Source) string {
	if s.Path != "" {
		return s.Path
	}
	return s.Name
}
