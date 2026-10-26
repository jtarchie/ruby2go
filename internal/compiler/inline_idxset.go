package compiler

import (
	"go/ast"
	"go/token"
)

// inlineIdxSet: Array#[]= costs Go's inliner over its budget (the slow path's generic call alone is ~70), so a statement `Array_Op_idxSet[T](a, i, v)` gets the in-bounds store at the site and calls it only otherwise (decision 387).
func inlineIdxSet(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		var list []ast.Stmt
		switch b := n.(type) {
		case *ast.BlockStmt:
			list = b.List
		case *ast.CaseClause:
			list = b.Body
		case *ast.CommClause:
			list = b.Body
		default:
			return true
		}
		for i, s := range list {
			if r := idxSetFast(s); r != nil {
				list[i] = r
			}
		}
		return true
	})
}

func idxSetFast(s ast.Stmt) ast.Stmt {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 3 || call.Ellipsis.IsValid() {
		return nil
	}
	ix, ok := call.Fun.(*ast.IndexExpr)
	if !ok || !isIdent(ix.X, "Array_Op_idxSet") || isIdent(call.Args[0], "rbIdxA") { // rbIdxA: the slow call already rewritten
		return nil
	}
	pos := call.Pos()
	id := func(name string) *ast.Ident { return &ast.Ident{NamePos: pos, Name: name} }
	a, i, v := "rbIdxA", "rbIdxI", "rbIdxV"
	sel := func(x, f string) ast.Expr { return &ast.SelectorExpr{X: id(x), Sel: id(f)} }
	conv := func(t string, x ast.Expr) ast.Expr {
		return &ast.CallExpr{Fun: id(t), Lparen: pos, Args: []ast.Expr{x}, Rparen: pos}
	}
	inBounds := &ast.BinaryExpr{X: conv("uint64", id(i)), OpPos: pos, Op: token.LSS, Y: conv("uint64", conv("len", sel(a, "s")))}
	notFrozen := &ast.UnaryExpr{OpPos: pos, Op: token.NOT, X: &ast.CallExpr{Fun: &ast.SelectorExpr{X: sel(a, "frozen"), Sel: id("Load")}, Lparen: pos, Rparen: pos}}
	store := &ast.AssignStmt{Lhs: []ast.Expr{&ast.IndexExpr{X: sel(a, "s"), Lbrack: pos, Index: id(i), Rbrack: pos}}, TokPos: pos, Tok: token.ASSIGN, Rhs: []ast.Expr{id(v)}}
	slow := &ast.ExprStmt{X: &ast.CallExpr{Fun: call.Fun, Lparen: pos, Args: []ast.Expr{id(a), id(i), id(v)}, Rparen: pos}}
	return &ast.IfStmt{
		If:   pos,
		Init: &ast.AssignStmt{Lhs: []ast.Expr{id(a), id(i), id(v)}, TokPos: pos, Tok: token.DEFINE, Rhs: []ast.Expr{call.Args[0], typedConst(call.Args[1], id("Integer")), typedConst(call.Args[2], ix.Index)}},
		Cond: &ast.BinaryExpr{X: inBounds, OpPos: pos, Op: token.LAND, Y: notFrozen},
		Body: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{store}, Rbrace: pos},
		Else: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{slow}, Rbrace: call.End()},
	}
}

func isIdent(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == name
}

// typedConst gives an untyped constant argument its parameter's type, which a := would make Go's int or reject (nil).
func typedConst(x, t ast.Expr) ast.Expr {
	switch v := x.(type) {
	case *ast.BasicLit:
	case *ast.UnaryExpr:
		if _, ok := v.X.(*ast.BasicLit); !ok {
			return x
		}
	case *ast.Ident:
		if v.Name != "nil" && v.Name != "true" && v.Name != "false" {
			return x
		}
	default:
		return x
	}
	if _, ok := t.(*ast.Ident); !ok {
		t = &ast.ParenExpr{X: t}
	}
	return &ast.CallExpr{Fun: t, Args: []ast.Expr{x}}
}
