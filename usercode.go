package rb2go

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

// UserCode is the part of Compile's output that mainName produced, for display: decls a `//line mainName:` directive precedes, of a decl with one inside (the top-level function) the part from there on, and the plumbing generated for mainName's types, found by name (X, XI, NewX, X_..., super_X_, receivers of those).
func UserCode(code []byte, mainName string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", code, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return string(code)
	}
	tf := fset.File(f.Pos())
	marker := []byte("//line " + mainName + ":")
	type piece struct {
		text   string
		names  []string
		marked bool
	}
	var pieces []piece
	var owners []string
	prev := tf.Offset(f.Name.End())
	for _, d := range f.Decls {
		start, end := tf.Offset(d.Pos()), tf.Offset(d.End())
		gap, body := code[prev:start], code[start:end]
		prev = end
		pc := piece{text: string(body), names: declNames(d)}
		if i := bytes.LastIndex(gap, marker); i >= 0 && !bytes.Contains(gap[i+len(marker):], []byte("//line ")) {
			pc.text, pc.marked = string(bytes.TrimSpace(gap[i:]))+"\n"+pc.text, true
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.TYPE {
				for _, n := range pc.names {
					owners = append(owners, strings.TrimSuffix(n, "_Self")) // a module's constraint is M_Self; its other decls are M_...
				}
			}
		} else if i := bytes.Index(body, marker); i >= 0 {
			header := body[:bytes.IndexByte(body, '\n')+1]
			if bytes.Contains(body[len(header):i], []byte("//line ")) {
				header = append(header[:len(header):len(header)], "\t// … prelude setup\n"...)
			}
			pc.text, pc.marked = string(header)+string(body[i:]), true
		}
		pieces = append(pieces, pc)
	}
	owned := func(n string) bool {
		return slices.ContainsFunc(owners, func(o string) bool {
			return n == o || n == o+"I" || n == "New"+o || n == "super_"+o+"_" || strings.HasPrefix(n, o+"_")
		})
	}
	var out []string
	for _, pc := range pieces {
		if pc.marked || slices.ContainsFunc(pc.names, owned) {
			out = append(out, pc.text)
		}
	}
	return strings.Join(out, "\n\n") + "\n"
}

// declNames is what a decl declares, a method by its receiver's type.
func declNames(d ast.Decl) []string {
	var out []string
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil {
			return []string{d.Name.Name}
		}
		t := d.Recv.List[0].Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		if id, ok := t.(*ast.Ident); ok {
			out = append(out, id.Name)
		}
	case *ast.GenDecl:
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				out = append(out, s.Name.Name)
			case *ast.ValueSpec:
				for _, n := range s.Names {
					out = append(out, n.Name)
				}
			}
		}
	}
	return out
}
