// Package boot runs a Ruby program's load phase under MRI at compile time and
// records what the source alone does not show: methods built by define_method
// or a string eval, the files autoload actually loaded, the classes and their
// ancestors, and the objects held in class state (docs/design.md decision
// 161). The compiled binary still runs no eval or define_method; only boot is
// evaluated, and only for a program that requires a gem.
package boot

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed boot_hook.rb find_gems.rb
var hookFS embed.FS

// Manifest is what one boot recorded.
type Manifest struct {
	LoadedFeatures []string         `json:"loaded_features"` // files loaded during boot
	Classes        []ClassInfo      `json:"classes"`
	DefineMethods  []MethodInfo     `json:"define_methods"`
	Evals          []EvalInfo       `json:"evals"`
	Autoloads      []AutoloadInfo   `json:"autoloads"`
	ClassState     []ClassStateInfo `json:"class_state"`
}

// ClassInfo is a class or module defined during boot.
type ClassInfo struct {
	Name      string   `json:"name"`
	Super     string   `json:"super"`
	Ancestors []string `json:"ancestors,omitempty"`
	Path      string   `json:"path"`
	Line      int      `json:"line"`
}

// MethodInfo is a method made by define_method (or from an existing
// UnboundMethod): Source is the block's text, nil for the latter.
type MethodInfo struct {
	Owner  string `json:"owner"`
	Name   string `json:"name"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Source string `json:"source"`
	From   string `json:"from"`
}

// EvalInfo is a string eval: Source is the evaluated text.
type EvalInfo struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Source string `json:"source"`
	Caller string `json:"caller"`
}

// AutoloadInfo is an autoload registration and whether boot resolved it.
type AutoloadInfo struct {
	Owner    string `json:"owner"`
	Const    string `json:"const"`
	Path     string `json:"path"`
	Resolved bool   `json:"resolved"`
	Loaded   string `json:"loaded"`
}

// ClassStateInfo is one instance variable held on a class/module object.
type ClassStateInfo struct {
	Owner    string `json:"owner"`
	Ivar     string `json:"ivar"`
	Kind     string `json:"kind"`
	Desc     string `json:"desc"`
	Recreate bool   `json:"recreate"` // a Mutex/Queue: recreated empty, not copied
	Frozen   bool   `json:"frozen"`
}

// Capture runs entry under MRuby with the tracing hook installed, with
// loadPaths as `-I` directories, and returns the boot manifest.
func Capture(ctx context.Context, entry string, loadPaths ...string) (*Manifest, error) {
	dir, err := os.MkdirTemp("", "rb2go-boot-")
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	hook, err := hookFS.ReadFile("boot_hook.rb")
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	hookPath := filepath.Join(dir, "boot_hook.rb")
	err = os.WriteFile(hookPath, hook, 0o600)
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	out := filepath.Join(dir, "manifest.json")
	args := []string{"-I", dir, "-r", "boot_hook"}
	for _, lp := range loadPaths {
		args = append(args, "-I", lp)
	}
	args = append(args, entry)
	cmd := exec.CommandContext(ctx, "ruby", args...) //nolint:gosec // ruby runs the program's own boot
	cmd.Env = append(os.Environ(), "RB2GO_BOOT_OUT="+out)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("boot %s: %w\n%s", entry, err, stderr.String())
	}
	data, err := os.ReadFile(out) //nolint:gosec // under os.MkdirTemp
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	var m Manifest
	err = json.Unmarshal(data, &m)
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	return &m, nil
}

// FindGems asks MRI, once, for the lib dirs of the installed gems providing names and their runtime dependencies, through Bundler when BUNDLE_GEMFILE is set or dir holds a Gemfile.
func FindGems(ctx context.Context, dir string, names []string) ([]string, error) {
	script, err := hookFS.ReadFile("find_gems.rb")
	if err != nil {
		return nil, fmt.Errorf("find gems: %w", err)
	}
	args := []string{"-e", string(script), "--"}
	env := os.Environ()
	gemfile := os.Getenv("BUNDLE_GEMFILE")
	if gemfile == "" {
		_, serr := os.Stat(filepath.Join(dir, "Gemfile"))
		if serr == nil {
			gemfile = filepath.Join(dir, "Gemfile")
			env = append(env, "BUNDLE_GEMFILE="+gemfile)
		}
	}
	if gemfile != "" {
		args = append([]string{"-rbundler/setup"}, args...)
	}
	cmd := exec.CommandContext(ctx, "ruby", append(args, names...)...) //nolint:gosec // names are the program's require literals
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("require %q: finding an installed gem needs ruby on PATH", names[0])
	}
	if err != nil {
		return nil, fmt.Errorf("%s (%w)", strings.TrimSpace(stderr.String()), err)
	}
	return strings.FieldsFunc(string(out), func(r rune) bool { return r == '\n' }), nil
}

// Class returns the named class, or nil.
func (m *Manifest) Class(name string) *ClassInfo {
	for i := range m.Classes {
		if m.Classes[i].Name == name {
			return &m.Classes[i]
		}
	}
	return nil
}

// DefinedMethods returns the methods define_method made on owner.
func (m *Manifest) DefinedMethods(owner string) []MethodInfo {
	var out []MethodInfo
	for _, d := range m.DefineMethods {
		if d.Owner == owner {
			out = append(out, d)
		}
	}
	return out
}
