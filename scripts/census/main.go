// Command census counts what a generated main.go spends its functions on (#96): forwarders, Dyn wrappers, stubs, markers, bodies, and how many forwarders sit on names with one implementation.
//
//	go -C scripts/census run . /path/to/main.go
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
)

func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return x.X.(*ast.Ident).Name
	case *ast.IndexListExpr:
		return x.X.(*ast.Ident).Name
	}
	return "?"
}

func calleeGeneric(e ast.Expr) (string, bool) {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	switch f := c.Fun.(type) {
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return id.Name, true
		}
	case *ast.IndexListExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return id.Name, true
		}
	case *ast.Ident:
		return f.Name, true
	case *ast.SelectorExpr: // T.Method(self,...) method expression
		return "sel:" + f.Sel.Name, true
	}
	return "", false
}

func main() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, os.Args[1], nil, parser.SkipObjectResolution)
	if err != nil {
		panic(err)
	}
	counts := map[string]int{}
	lines := map[string]int{}
	perRecvFwd := map[string]int{}
	fwdTargets := map[string]int{}
	metaCounts := map[string]int{}
	chaReport(fset, f)
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		n := fset.Position(fd.End()).Line - fset.Position(fd.Pos()).Line + 1
		kind := ""
		name := fd.Name.Name
		r := recvName(fd)
		meta := strings.HasSuffix(r, "_Meta")
		if fd.Recv == nil {
			switch {
			case fd.Type.TypeParams != nil:
				kind = "free.generic"
			case strings.HasPrefix(name, "rbDyn"):
				kind = "free.rbDyn-dispatcher"
			default:
				kind = "free.plain"
			}
		} else {
			body := fd.Body.List
			switch {
			case strings.HasPrefix(name, "Dyn") || strings.HasPrefix(name, "_Dyn"):
				kind = "method.Dyn-wrapper"
			case len(body) == 1 && isPruned(body[0]):
				kind = "method.pruned-stub"
			case len(body) == 0:
				kind = "method.empty-marker"
			case len(body) == 1 && isFwd(body[0], fwdTargets):
				kind = "method.forwarder"
				perRecvFwd[r]++
			case len(body) == 1 && isConstReturn(body[0]):
				kind = "method.const-return"
			default:
				kind = "method.body"
			}
			if meta {
				metaCounts[kind]++
			}
		}
		counts[kind]++
		lines[kind] += n
	}
	types := map[string]int{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, s := range gd.Specs {
			ts := s.(*ast.TypeSpec)
			k := fmt.Sprintf("%T", ts.Type)
			if ts.TypeParams != nil {
				k += "+generic"
			}
			if strings.HasSuffix(ts.Name.Name, "_Meta") {
				k += " (meta)"
			}
			if it, ok := ts.Type.(*ast.InterfaceType); ok {
				types["iface.methods"] += len(it.Methods.List)
			}
			types[k]++
		}
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%-28s %8d funcs %9d lines  meta:%d\n", k, counts[k], lines[k], metaCounts[k])
	}
	fmt.Println("types:", types)
	type kv struct {
		k string
		v int
	}
	var tv []kv
	for k, v := range fwdTargets {
		tv = append(tv, kv{k, v})
	}
	sort.Slice(tv, func(i, j int) bool { return tv[i].v > tv[j].v })
	fmt.Println("top forwarder targets:")
	for _, x := range tv[:min(40, len(tv))] {
		fmt.Printf("  %6d %s\n", x.v, x.k)
	}
	fmt.Println("distinct forwarder targets:", len(tv))
	var rv []kv
	for k, v := range perRecvFwd {
		rv = append(rv, kv{k, v})
	}
	sort.Slice(rv, func(i, j int) bool { return rv[i].v > rv[j].v })
	fmt.Println("receivers with forwarders:", len(rv))
	for _, x := range rv[:min(15, len(rv))] {
		fmt.Printf("  %6d %s\n", x.v, x.k)
	}
}

func isPruned(s ast.Stmt) bool {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return false
	}
	c, ok := es.X.(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok || id.Name != "panic" {
		return false
	}
	bl, ok := c.Args[0].(*ast.BasicLit)
	return ok && strings.Contains(bl.Value, "pruned")
}

func isConstReturn(s ast.Stmt) bool {
	rs, ok := s.(*ast.ReturnStmt)
	if !ok || len(rs.Results) != 1 {
		return false
	}
	switch rs.Results[0].(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return true
	}
	return false
}

func isFwd(s ast.Stmt, tgt map[string]int) bool {
	var e ast.Expr
	switch x := s.(type) {
	case *ast.ReturnStmt:
		if len(x.Results) != 1 {
			return false
		}
		e = x.Results[0]
	case *ast.ExprStmt:
		e = x.X
	default:
		return false
	}
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) == 0 {
		return false
	}
	// first arg must be self (possibly converted)
	if !mentionsSelf(c.Args[0]) {
		return false
	}
	name, ok := calleeGeneric(c)
	if !ok {
		return false
	}
	tgt[name]++
	return true
}

func mentionsSelf(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "self" {
			found = true
		}
		return !found
	})
	return found
}
