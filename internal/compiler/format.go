package compiler

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
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
	"io": "io", "iter": "iter", "json": "encoding/json", "maphash": "hash/maphash", "maps": "maps", "math": "math", "mime": "mime", "tls": "crypto/tls",
	"net": "net", "netip": "net/netip", "os": "os", "rand": "crypto/rand", "regexp": "regexp",
	"runtime": "runtime", "signal": "os/signal", "syscall": "syscall", "slices": "slices", "sort": "sort", "strconv": "strconv",
	"strings": "strings", "sync": "sync", "atomic": "sync/atomic",
	"base64": "encoding/base64", "hex": "encoding/hex", "md5": "crypto/md5", "sha1": "crypto/sha1",
	"sha256": "crypto/sha256", "sha512": "crypto/sha512", "crc32": "hash/crc32", "syntax": "regexp/syntax",
	"gzip": "compress/gzip", "zlib": "compress/zlib",
	"time": "time", "unicode": "unicode", "unsafe": "unsafe", "weak": "weak", "url": "net/url", "user": "os/user",
	"utf16": "unicode/utf16", "utf8": "unicode/utf8",
}

// errPruneIncomplete: a dispatcher became reachable after the tables went out (a type-switch case unlocked late); the caller recompiles emitting every one first.
var errPruneIncomplete = errors.New("pruning reached dispatchers emitted lazily")

// addFrameLabels emits rbFrameLabels, the Go function → Ruby label table
// Exception#backtrace reads (decision 106), when the pruned program kept
// its reader: one entry per kept function the compiler labelled.
func addFrameLabels(fset *token.FileSet, f *ast.File, labels map[string]string) error {
	kept := map[string]bool{}
	reader := false
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fd.Name.Name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			name = recvName(fd.Recv.List[0].Type) + "." + name
		}
		kept[name] = true
		if name == "rbBacktraceFrames" {
			reader = true
		}
	}
	if !reader {
		return nil
	}
	var b strings.Builder
	b.WriteString("package main\n\nvar rbFrameLabels = map[string]string{\n")
	for _, k := range slices.Sorted(maps.Keys(labels)) {
		if kept[k] {
			fmt.Fprintf(&b, "\t%q: %q,\n", k, labels[k])
		}
	}
	b.WriteString("}\n")
	t, err := parser.ParseFile(fset, "labels.go", b.String(), parser.ParseComments)
	if err != nil {
		return fmt.Errorf("gofmt: %w", err)
	}
	merge(f, t)
	return nil
}

// merge appends t's declarations and comments to f: both came from one FileSet, t after f, so positions stay in order for the printer.
func merge(f, t *ast.File) {
	f.Decls = append(f.Decls, t.Decls...)
	f.Comments = append(f.Comments, t.Comments...)
}

// formatGo prunes src to what main reaches (RB2GO_NO_PRUNE=1 keeps everything), adds the std imports it refers to, and gofmts it. src is the program without its forwarders, dispatchers and tables: next emits those in batches for what the pruned program so far reaches (most are never reached), until it returns nil.
func formatGo(src []byte, lazy map[string]bool, next func(reached, selected func(string) bool) ([]byte, error), labels map[string]string) ([]byte, error) {
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
		if os.Getenv("RB2GO_PRUNE_WHY") != "" {
			p.explain(f, os.Stderr)
		}
	}
	err = addFrameLabels(fset, f, labels)
	if err != nil {
		return nil, err
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
	out := tidyBraces(buf.Bytes())
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

// tidyBraces drops the blank line pruning leaves after `{` (a dropped first type-switch case) or before `}` (a switch shortened inside its block); the scanner keeps braces inside literals out of it.
func tidyBraces(src []byte) []byte {
	var sc scanner.Scanner
	file := token.NewFileSet().AddFile("", -1, len(src))
	sc.Init(file, src, nil, 0)
	var cuts []int
	for {
		pos, tok, _ := sc.Scan()
		if tok == token.EOF {
			break
		}
		o := file.Offset(pos)
		switch {
		case tok == token.LBRACE && o+2 < len(src) && src[o+1] == '\n' && src[o+2] == '\n':
			cuts = append(cuts, o+1)
		case tok == token.RBRACE:
			i := o - 1
			for i >= 0 && (src[i] == '\t' || src[i] == ' ') {
				i--
			}
			if i >= 1 && src[i] == '\n' && src[i-1] == '\n' {
				cuts = append(cuts, i)
			}
		}
	}
	if len(cuts) == 0 {
		return src
	}
	out := make([]byte, 0, len(src))
	prev := 0
	for _, c := range cuts {
		out = append(out, src[prev:c]...)
		prev = c + 1
	}
	return append(out, src[prev:]...)
}
