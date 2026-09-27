package compiler

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// stdImports replaces goimports, which runs the go command; a package missing here fails `go build` with "undefined: name".
var stdImports = map[string]string{
	"bits": "math/bits", "bufio": "bufio", "cmp": "cmp", "context": "context",
	"errors": "errors", "fmt": "fmt", "fnv": "hash/fnv", "http": "net/http",
	"io": "io", "iter": "iter", "maphash": "hash/maphash", "math": "math",
	"net": "net", "os": "os", "reflect": "reflect", "regexp": "regexp",
	"runtime": "runtime", "slices": "slices", "strconv": "strconv",
	"strings": "strings", "sync": "sync", "syntax": "regexp/syntax",
	"time": "time", "unicode": "unicode", "unsafe": "unsafe", "url": "net/url",
	"utf16": "unicode/utf16", "utf8": "unicode/utf8",
}

// formatGo adds the imports of the std packages src refers to, and gofmts it.
func formatGo(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
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
	var imp strings.Builder
	imp.WriteString("\nimport (\n")
	for _, p := range slices.Sorted(maps.Keys(used)) {
		imp.WriteString("\t" + strconv.Quote(p) + "\n")
	}
	imp.WriteString(")\n")
	at := fset.Position(f.Name.End()).Offset
	out, err := format.Source(slices.Concat(src[:at], []byte(imp.String()), src[at:]))
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
	}
	return out, nil
}
