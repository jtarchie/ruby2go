package compiler

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
)

// A gem method whose generated Go does not type-check is poisoned as one rb2go cannot compile is (decision 205): the
// traced build type-checks its output, poisons the methods Go rejected, and compiles again.

// methodKey names a method across compiles of the same sources.
func methodKey(m *Method) string {
	return fmt.Sprintf("%s:%d:%s", m.File.path, m.Line, m.Name)
}

// applyGoPoison poisons the methods an earlier compile's Go rejected.
func (c *Compiler) applyGoPoison(poison map[string]string) {
	if len(poison) == 0 {
		return
	}
	mark := func(m *Method) {
		if msg, ok := poison[methodKey(m)]; ok && m.File != nil && m.poison == "" {
			m.poison = msg
		}
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if m.File != nil {
				mark(m)
			}
		}
	}
}

// goTypeErrors type-checks generated Go and returns, by enclosing top-level func name, the first error in each
// (errors outside any func are returned under "").
func goTypeErrors(code []byte) map[string]string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", code, parser.SkipObjectResolution)
	if err != nil {
		return nil
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
	conf := types.Config{Importer: importer.Default(), Error: func(err error) {
		var te types.Error
		if !errors.As(err, &te) {
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
