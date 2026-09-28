package compiler

import (
	"bytes"
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
	"errors": "errors", "fmt": "fmt", "fnv": "hash/fnv", "fs": "io/fs", "filepath": "path/filepath", "http": "net/http",
	"io": "io", "iter": "iter", "maphash": "hash/maphash", "math": "math",
	"net": "net", "os": "os", "rand": "crypto/rand", "reflect": "reflect", "regexp": "regexp",
	"runtime": "runtime", "signal": "os/signal", "syscall": "syscall", "slices": "slices", "sort": "sort", "strconv": "strconv",
	"strings": "strings", "sync": "sync", "atomic": "sync/atomic",
	"base64": "encoding/base64", "hex": "encoding/hex", "md5": "crypto/md5", "sha1": "crypto/sha1",
	"sha256": "crypto/sha256", "sha512": "crypto/sha512", "crc32": "hash/crc32", "syntax": "regexp/syntax",
	"time": "time", "unicode": "unicode", "unsafe": "unsafe", "url": "net/url", "user": "os/user",
	"utf16": "unicode/utf16", "utf8": "unicode/utf8",
}

// formatGo prunes src to what main reaches (RB2GO_NO_PRUNE=1 keeps everything), adds the std imports it refers to, and gofmts it.
func formatGo(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
	}
	if os.Getenv("RB2GO_NO_PRUNE") == "" {
		pruneDecls(f)
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
