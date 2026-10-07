package compiler

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// loadSigs parses .rbs signature files beside each user source (a `sig/`
// directory next to the file), keyed by qualified class name (decision 160).
// Inline `#:` annotations win; a sig is consulted only where one is missing.
func (c *Compiler) loadSigs(sources []Source) {
	seen := map[string]bool{}
	for _, src := range sources {
		p := src.Path
		if p == "" {
			p = src.Name
		}
		dir := filepath.Join(filepath.Dir(p), "sig")
		if seen[dir] {
			continue
		}
		seen[dir] = true
		c.loadSigDir(dir)
	}
}

func (c *Compiler) loadSigDir(dir string) {
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".rbs") {
			return nil
		}
		data, rerr := os.ReadFile(path) //nolint:gosec // the program's own sig directory
		if rerr != nil {
			return fmt.Errorf("read %s: %w", path, rerr)
		}
		f, perr := rbs.ParseFile(string(data))
		if perr != nil {
			c.errorf(nil, nil, "%s: %v", path, perr)
		}
		for _, decl := range f.Decls {
			c.sigs[decl.Name] = decl
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		c.errorf(nil, nil, "%v", err)
	}
}

// sigMethod is the sig file's method `name` on cls (a `def self.` when cls is
// a metaclass), or nil.
func (c *Compiler) sigMethod(cls *Class, name string) *rbs.MethodDecl {
	if cls == nil {
		return nil
	}
	d := c.sigs[cls.RubyName]
	if d == nil {
		return nil
	}
	wantSelf := cls.metaOf != nil
	for _, m := range d.Methods {
		if m.Name == name && m.Self == wantSelf {
			return m
		}
	}
	return nil
}

// sigAttr is the sig file's attribute `name` on cls, of any kind.
func (c *Compiler) sigAttr(cls *Class, name string) *rbs.AttrDecl {
	if cls == nil {
		return nil
	}
	d := c.sigs[cls.RubyName]
	if d == nil {
		return nil
	}
	for _, a := range d.Attrs {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// sigTypeParams is the sig file's type parameters for cls, or nil.
func (c *Compiler) sigTypeParams(cls *Class) []string {
	if cls == nil {
		return nil
	}
	if d := c.sigs[cls.RubyName]; d != nil {
		return d.TypeParams
	}
	return nil
}
