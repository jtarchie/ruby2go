package compiler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jtarchie/ruby2go/internal/boot"
)

// CompileWithBoot is CompileFilesWithWarnings for a program that requires a
// gem: it runs the program's load phase under MRI first (internal/boot,
// decision 161) and adds each autoload boot resolved as a source, so the
// compiler sees classes the source graph's literal requires do not name. A
// program whose entry is not on disk is compiled as usual.
func CompileWithBoot(ctx context.Context, preludeFS fs.FS, sources []Source, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	aug, err := bootSources(ctx, sources, loadPath)
	if err != nil {
		return nil, nil, err
	}
	opts := options{loadPath: loadPath}
	out, err := compile(ctx, preludeFS, aug, &opts)
	return out, opts.warnings, err
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
