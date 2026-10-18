package compiler

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"sort"
	"strings"
)

// A gem method whose generated Go does not type-check is poisoned as one rb2go cannot compile is (decision 205): the
// traced build type-checks its output and replaces each body Go rejected with the NotImplementedError raise, in the
// Go text, keeping the signature so its callers still type-check (decision 303).

// methodKey names a method across compiles of the same sources.
func methodKey(m *Method) string {
	return fmt.Sprintf("%s:%d:%s", m.File.path, m.Line, m.Name)
}

// goFunc is a gem method's Go func: its methodKey and the name its NotImplementedError gives.
type goFunc struct{ key, name string }

// goTypeErrors type-checks generated Go and returns, by enclosing top-level func name, the first error in each
// (errors outside any func are returned under ""), and the byte offsets of the imports left unused.
func goTypeErrors(code []byte) (map[string]string, []int) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", code, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil
	}
	type span struct {
		from, to token.Pos
		name     string
	}
	var spans []span
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			spans = append(spans, span{fd.Pos(), fd.End(), funcDeclName(fd)})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	out := map[string]string{}
	var unused []int
	conf := types.Config{Importer: importer.Default(), Error: func(err error) {
		var te types.Error
		if !errors.As(err, &te) {
			return
		}
		if strings.HasSuffix(te.Msg, " imported and not used") {
			unused = append(unused, fset.Position(te.Pos).Offset)
			return
		}
		i := sort.Search(len(spans), func(i int) bool { return spans[i].to > te.Pos })
		name := ""
		if i < len(spans) && spans[i].from <= te.Pos {
			name = spans[i].name
		}
		if _, seen := out[name]; !seen {
			out[name] = te.Msg
		}
	}}
	_, _ = conf.Check("main", fset, []*ast.File{f}, nil)
	return out, unused
}

// stubRejected type-checks code and stubs each func Go rejects that stub answers for (its NotImplementedError
// message, true), until Go rejects no more of them.
func stubRejected(code []byte, stub func(fn, msg string) (string, bool)) []byte {
	stubbed := map[string]bool{}
	for range goPoisonRounds { // a stub reaches less, so a round after the first only drops what it left unused
		bad, unused := goTypeErrors(code) // nil when it does not parse: go build reports that, with the generated file kept
		stubs := map[string]string{}
		for fn, msg := range bad {
			if stubbed[fn] {
				continue
			}
			if text, ok := stub(fn, msg); ok {
				stubbed[fn] = true
				stubs[fn] = text
			}
		}
		if len(stubs) == 0 && len(unused) == 0 {
			break
		}
		code = stubGoFuncs(code, stubs, unused)
	}
	return code
}

// stubGoFuncs replaces the body of each func named in stubs with a NotImplementedError raise carrying its message,
// and deletes the lines of the imports at unused.
func stubGoFuncs(code []byte, stubs map[string]string, unused []int) []byte {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", code, parser.SkipObjectResolution)
	if err != nil {
		return code
	}
	type cut struct {
		from, to int
		with     string
	}
	var cuts []cut
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			if msg, ok := stubs[funcDeclName(fd)]; ok {
				cuts = append(cuts, cut{fset.Position(fd.Body.Lbrace).Offset + 1, fset.Position(fd.Body.Rbrace).Offset,
					fmt.Sprintf("\n\tpanic(NewNotImplementedError(Ref(String(%q))))\n", msg)})
			}
		}
	}
	for _, at := range unused {
		from := bytes.LastIndexByte(code[:at], '\n') + 1
		to := at + bytes.IndexByte(code[at:], '\n') + 1
		cuts = append(cuts, cut{from, to, ""})
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].from > cuts[j].from })
	out := slices.Clone(code)
	for _, c := range cuts {
		out = slices.Replace(out, c.from, c.to, []byte(c.with)...)
	}
	return out
}

// funcDeclName is a func's name, or Recv.Name for a method (`func (self *X[V]) M()` is X.M).
func funcDeclName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch x := t.(type) {
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}
