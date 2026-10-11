// Command inst walks a generated main.go's generic instantiations from its non-generic code and counts compiled bodies by GC shape (#96). A lower bound: it misses inferred type arguments.
//
//	go -C scripts/census run ./inst /path/to/main.go
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
)

var fset = token.NewFileSet()

func str(e ast.Expr) string {
	var b bytes.Buffer
	printer.Fprint(&b, fset, e)
	return b.String()
}

type ref struct {
	callee string
	args   []string
}

type generic struct {
	params []string
	refs   []ref
	isType bool
	nmeth  int
}

var typeKind = map[string]string{} // name -> "iface" | "struct" | underlying string

func shapeOf(arg string) string {
	arg = strings.TrimSpace(arg)
	if strings.HasPrefix(arg, "*") {
		return "ptr"
	}
	if arg == "any" || arg == "interface{}" {
		return "any"
	}
	base := arg
	if i := strings.Index(base, "["); i >= 0 {
		base = base[:i]
	}
	k, ok := typeKind[base]
	if !ok {
		return "other:" + arg
	}
	switch k {
	case "iface":
		return "iface:" + arg
	case "struct":
		return "struct:" + arg
	}
	return "u:" + k
}

func main() {
	f, err := parser.ParseFile(fset, os.Args[1], nil, parser.SkipObjectResolution)
	if err != nil {
		panic(err)
	}
	gens := map[string]*generic{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, s := range gd.Specs {
			ts := s.(*ast.TypeSpec)
			switch t := ts.Type.(type) {
			case *ast.InterfaceType:
				typeKind[ts.Name.Name] = "iface"
			case *ast.StructType:
				typeKind[ts.Name.Name] = "struct"
			default:
				typeKind[ts.Name.Name] = str(t)
			}
			if ts.TypeParams != nil {
				g := &generic{isType: true}
				for _, fl := range ts.TypeParams.List {
					for _, n := range fl.Names {
						g.params = append(g.params, n.Name)
					}
				}
				gens[ts.Name.Name] = g
			}
		}
	}
	// resolve named non-struct underlying chains roughly
	for k, v := range typeKind {
		if v != "iface" && v != "struct" {
			if u, ok := typeKind[v]; ok && (u == "iface" || u == "struct") {
				typeKind[k] = u
			}
		}
	}
	collect := func(body ast.Node) []ref {
		var rs []ref
		ast.Inspect(body, func(n ast.Node) bool {
			var x ast.Expr
			var idx []ast.Expr
			switch e := n.(type) {
			case *ast.IndexExpr:
				x, idx = e.X, []ast.Expr{e.Index}
			case *ast.IndexListExpr:
				x, idx = e.X, e.Indices
			default:
				return true
			}
			id, ok := x.(*ast.Ident)
			if !ok {
				return true
			}
			r := ref{callee: id.Name}
			for _, a := range idx {
				r.args = append(r.args, str(a))
			}
			rs = append(rs, r)
			return true
		})
		return rs
	}
	var roots []ref
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			// var/const decls at top level
			roots = append(roots, collect(d)...)
			continue
		}
		var owner string
		var params []string
		if fd.Recv != nil {
			t := fd.Recv.List[0].Type
			if s, ok := t.(*ast.StarExpr); ok {
				t = s.X
			}
			switch x := t.(type) {
			case *ast.IndexExpr:
				owner = x.X.(*ast.Ident).Name
				params = []string{x.Index.(*ast.Ident).Name}
			case *ast.IndexListExpr:
				owner = x.X.(*ast.Ident).Name
				for _, i := range x.Indices {
					params = append(params, i.(*ast.Ident).Name)
				}
			}
		} else if fd.Type.TypeParams != nil {
			owner = fd.Name.Name
			for _, fl := range fd.Type.TypeParams.List {
				for _, n := range fl.Names {
					params = append(params, n.Name)
				}
			}
			gens[owner] = &generic{params: params}
		}
		rs := collect(fd)
		if owner == "" {
			roots = append(roots, rs...)
			continue
		}
		g := gens[owner]
		if g.isType {
			g.nmeth++
			// rename method's receiver params to the type's declared params
			for i := range rs {
				for j := range rs[i].args {
					rs[i].args[j] = subst(rs[i].args[j], params, g.params)
				}
			}
		}
		g.refs = append(g.refs, rs...)
	}
	// worklist over instantiations
	type inst struct{ callee, key string }
	seen := map[inst]bool{}
	seenExact := map[string]bool{}
	var work []ref
	for _, r := range roots {
		if _, ok := gens[r.callee]; ok {
			work = append(work, r)
		}
	}
	byShapeKind := map[string]int{}
	typeWrappers := 0
	bodies := map[string]int{} // callee -> distinct shape instantiations
	for len(work) > 0 {
		r := work[len(work)-1]
		work = work[:len(work)-1]
		g := gens[r.callee]
		if len(r.args) != len(g.params) {
			continue
		}
		ek := r.callee + "[" + strings.Join(r.args, ",") + "]"
		if seenExact[ek] {
			continue
		}
		seenExact[ek] = true
		if g.isType && !strings.ContainsAny(strings.Join(r.args, ","), "\x00") {
			typeWrappers += g.nmeth
		}
		if len(seenExact) > 3_000_000 {
			fmt.Println("explosion; stopping")
			break
		}
		var shapes []string
		kinds := []string{}
		for _, a := range r.args {
			s := shapeOf(a)
			shapes = append(shapes, s)
			kinds = append(kinds, strings.SplitN(s, ":", 2)[0])
		}
		k := inst{r.callee, strings.Join(shapes, "|")}
		if !seen[k] {
			seen[k] = true
			n := 1
			if g.isType {
				n = g.nmeth
			}
			bodies[r.callee] += n
			byShapeKind[strings.Join(kinds, ",")] += n
		}
		for _, cr := range g.refs {
			if _, ok := gens[cr.callee]; !ok {
				continue
			}
			nr := ref{callee: cr.callee}
			for _, a := range cr.args {
				nr.args = append(nr.args, subst(a, g.params, r.args))
			}
			work = append(work, nr)
		}
	}
	total := 0
	for _, v := range bodies {
		total += v
	}
	fmt.Println("exact instantiations:", len(seenExact), " compiled bodies (by shape):", total, " generic-type method wrappers (per concrete type):", typeWrappers)
	type kv struct {
		k string
		v int
	}
	var sk []kv
	for k, v := range byShapeKind {
		sk = append(sk, kv{k, v})
	}
	sort.Slice(sk, func(i, j int) bool { return sk[i].v > sk[j].v })
	fmt.Println("bodies by arg-shape kind:")
	for _, x := range sk[:min(15, len(sk))] {
		fmt.Printf("  %7d %s\n", x.v, x.k)
	}
	var bk []kv
	for k, v := range bodies {
		bk = append(bk, kv{k, v})
	}
	sort.Slice(bk, func(i, j int) bool { return bk[i].v > bk[j].v })
	fmt.Println("top generic callees by bodies:")
	for _, x := range bk[:min(25, len(bk))] {
		fmt.Printf("  %7d %s\n", x.v, x.k)
	}
	// what if every iface Self were collapsed to one shape (e.g. instantiate with *T or any)?
	collapsed := map[string]bool{}
	ctotal := 0
	for k := range seen {
		parts := strings.Split(k.key, "|")
		for i, p := range parts {
			if strings.HasPrefix(p, "iface:") || strings.HasPrefix(p, "struct:") {
				parts[i] = "ptr"
			}
		}
		ck := k.callee + "|" + strings.Join(parts, "|")
		if !collapsed[ck] {
			collapsed[ck] = true
			n := 1
			if gens[k.callee].isType {
				n = gens[k.callee].nmeth
			}
			ctotal += n
		}
	}
	fmt.Println("bodies if interface/struct Self args collapsed to one pointer shape:", ctotal)
}

var wordRe = map[string]*regexp.Regexp{}

func subst(s string, from, to []string) string {
	if len(from) == 0 {
		return s
	}
	// single pass with placeholders
	for i, p := range from {
		re, ok := wordRe[p]
		if !ok {
			re = regexp.MustCompile(`\b` + regexp.QuoteMeta(p) + `\b`)
			wordRe[p] = re
		}
		s = re.ReplaceAllString(s, fmt.Sprintf("\x00%d\x00", i))
	}
	for i := range from {
		if i < len(to) {
			s = strings.ReplaceAll(s, fmt.Sprintf("\x00%d\x00", i), to[i])
		}
	}
	return s
}
