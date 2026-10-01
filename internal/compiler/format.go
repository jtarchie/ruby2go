package compiler

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
)

// stdImports replaces goimports, which runs the go command; a package missing here fails `go build` with "undefined: name".
var stdImports = map[string]string{
	"big": "math/big", "bits": "math/bits", "binary": "encoding/binary", "bufio": "bufio", "bytes": "bytes", "cmp": "cmp", "context": "context",
	"errors": "errors", "exec": "os/exec", "fmt": "fmt", "fnv": "hash/fnv", "fs": "io/fs", "filepath": "path/filepath", "http": "net/http",
	"io": "io", "iter": "iter", "json": "encoding/json", "maphash": "hash/maphash", "math": "math",
	"net": "net", "netip": "net/netip", "os": "os", "rand": "crypto/rand", "regexp": "regexp",
	"runtime": "runtime", "signal": "os/signal", "syscall": "syscall", "slices": "slices", "sort": "sort", "strconv": "strconv",
	"strings": "strings", "sync": "sync", "atomic": "sync/atomic",
	"base64": "encoding/base64", "hex": "encoding/hex", "md5": "crypto/md5", "sha1": "crypto/sha1",
	"sha256": "crypto/sha256", "sha512": "crypto/sha512", "crc32": "hash/crc32", "syntax": "regexp/syntax",
	"gzip": "compress/gzip", "zlib": "compress/zlib",
	"time": "time", "unicode": "unicode", "unsafe": "unsafe", "url": "net/url", "user": "os/user",
	"utf16": "unicode/utf16", "utf8": "unicode/utf8",
}

// errPruneIncomplete: a dispatcher became reachable after the tables went out (a type-switch case unlocked late); the caller recompiles emitting every one first.
var errPruneIncomplete = errors.New("pruning reached dispatchers emitted lazily")

// merge appends t's declarations and comments to f: both came from one FileSet, t after f, so positions stay in order for the printer.
func merge(f, t *ast.File) {
	f.Decls = append(f.Decls, t.Decls...)
	f.Comments = append(f.Comments, t.Comments...)
}

// formatGo prunes src to what main reaches (RB2GO_NO_PRUNE=1 keeps everything), adds the std imports it refers to, and gofmts it. src is the program without its forwarders, dispatchers and tables: next emits those in batches for what the pruned program so far reaches (most are never reached), until it returns nil.
func formatGo(src []byte, lazy map[string]bool, next func(reached, selected func(string) bool) ([]byte, error)) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
	}
	all := func(string) bool { return true }
	var p *pruner
	reached, selected := all, all
	if os.Getenv("RB2GO_NO_PRUNE") == "" {
		p = newPruner(lazy)
		p.add(f.Decls)
		p.run()
		reached, selected = p.reached, p.selected
	}
	for i := 0; ; i++ {
		more, err := next(reached, selected)
		if err != nil {
			return nil, err
		}
		if more == nil {
			break
		}
		t, err := parser.ParseFile(fset, fmt.Sprintf("tail%d.go", i), append([]byte("package main\n"), more...), parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("gofmt: %w", err)
		}
		merge(f, t)
		if p != nil {
			p.add(t.Decls)
			p.run()
		}
	}
	if p != nil {
		p.sweep(f)
	}
	used := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil && stdImports[id.Name] != "" {
				used[stdImports[id.Name]] = true
			}
		}
		return true
	})
	var buf bytes.Buffer
	err = format.Node(&buf, fset, f)
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
	}
	out := buf.Bytes()
	if len(used) == 0 {
		return out, nil
	}
	// the printed file is gofmt's: `package x`, a blank line, the decls; the import block goes between
	var imp strings.Builder
	imp.WriteString("\nimport (\n")
	for _, p := range slices.Sorted(maps.Keys(used)) {
		imp.WriteString("\t" + strconv.Quote(p) + "\n")
	}
	imp.WriteString(")\n")
	pkg := bytes.Index(out, []byte("package "+f.Name.Name+"\n"))
	at := pkg + len("package "+f.Name.Name+"\n")
	return slices.Concat(out[:at], []byte(imp.String()), out[at:]), nil
}
