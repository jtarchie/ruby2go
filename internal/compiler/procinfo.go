package compiler

import (
	"strconv"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// procParam is one entry of Proc#parameters: its kind and name ("" for an unnamed destructuring one).
type procParam struct {
	kind, name string
	required   bool // a leading or trailing positional name, which a proc reports as :opt
}

// procLiteralParams is a receiver that is a Proc literal: whether it is a lambda, and its parameter list node.
func procLiteralParams(n parser.Node) (lambda bool, params parser.Node, ok bool) {
	switch r := n.(type) {
	case *parser.LambdaNode:
		return true, r.Parameters, true
	case *parser.CallNode:
		b, isBlock := r.Block.(*parser.BlockNode)
		named := r.Receiver == nil && (r.Name == "proc" || r.Name == "lambda") || r.Name == "new" && isTopConst(r.Receiver, "Proc")
		if !isBlock || r.Arguments != nil || !named {
			return false, nil, false
		}
		return r.Name == "lambda", b.Parameters, true
	}
	return false, nil, false
}

// procParams is MRI's Proc#parameters for a literal's parameter list.
func procParams(lambda bool, params parser.Node, moved map[*parser.BlockParametersNode]int) []procParam {
	req := map[bool]string{true: "req", false: "opt"}[lambda]
	var out []procParam
	switch p := params.(type) {
	case *parser.NumberedParametersNode:
		for i := range int(p.Maximum) {
			out = append(out, procParam{req, "_" + strconv.Itoa(i+1), true})
		}
		return out
	case *parser.ItParametersNode:
		return []procParam{{req, "", true}}
	case *parser.BlockParametersNode:
		if p.Parameters == nil {
			return nil
		}
		ps := p.Parameters
		named := func(kind string, nodes []parser.Node, required bool) {
			for _, x := range nodes {
				name := ""
				switch v := x.(type) {
				case *parser.RequiredParameterNode:
					name = v.Name
				case *parser.OptionalParameterNode:
					name = v.Name
				}
				out = append(out, procParam{kind, name, required})
			}
		}
		k := len(ps.Requireds) - moved[p] // the last moved ones were `|x = 1|`
		named(req, ps.Requireds[:k], true)
		named("opt", ps.Requireds[k:], false)
		named("opt", ps.Optionals, false)
		if r, ok := ps.Rest.(*parser.RestParameterNode); ok {
			out = append(out, procParam{kind: "rest", name: derefName(r.Name, "*")})
		}
		named(req, ps.Posts, true)
		for _, k := range ps.Keywords {
			switch v := k.(type) {
			case *parser.RequiredKeywordParameterNode:
				out = append(out, procParam{kind: "keyreq", name: v.Name})
			case *parser.OptionalKeywordParameterNode:
				out = append(out, procParam{kind: "key", name: v.Name})
			}
		}
		switch v := ps.KeywordRest.(type) {
		case *parser.KeywordRestParameterNode:
			out = append(out, procParam{kind: "keyrest", name: derefName(v.Name, "**")})
		case *parser.NoKeywordsParameterNode:
			out = append(out, procParam{kind: "nokey"})
		}
		if ps.Block != nil {
			out = append(out, procParam{kind: "block", name: derefName(ps.Block.Name, "&")})
		}
	}
	return out
}

func derefName(p *string, anon string) string {
	if p == nil {
		return anon
	}
	return *p
}

// procArity is MRI's Proc#arity from its parameters: a lambda's optional ones make it -(required+1), a proc's only a rest does.
func procArity(lambda bool, ps []procParam) int {
	req, opt, rest, keyreq, key := 0, false, false, false, false
	for _, p := range ps {
		switch p.kind {
		case "req":
			req++
		case "opt":
			if p.required {
				req++ // a proc's leading names count, though they are optional
			} else {
				opt = true
			}
		case "rest":
			rest = true
		case "keyreq":
			keyreq = true
		case "key":
			key = true
		}
	}
	if keyreq {
		req++
	}
	if rest || lambda && (opt || key && !keyreq) {
		return -(req + 1)
	}
	return req
}

// genProcInfo is `lambda?`, `arity` or `parameters` on a Proc literal: answered from its source, unbuilt (decision 364).
func (f *fctx) genProcInfo(n *parser.CallNode) (expr, bool) {
	if n.Receiver == nil || n.Arguments != nil || n.Block != nil || n.Name != "lambda?" && n.Name != "arity" && n.Name != "parameters" {
		return expr{}, false
	}
	lambda, params, ok := procLiteralParams(n.Receiver)
	if !ok {
		return expr{}, false
	}
	if o := f.f.procArgsOrig[asBlockParams(params)]; o != nil {
		params = o
	}
	ps := procParams(lambda, params, f.f.movedOpts)
	switch n.Name {
	case "lambda?":
		return expr{code: strconv.FormatBool(lambda), typ: f.cls("Boolean"), lit: true}, true
	case "arity":
		return expr{code: strconv.Itoa(procArity(lambda, ps)), typ: f.cls("Integer"), lit: true}, true
	}
	loc := n.Location
	sym := func(s string) parser.Node {
		return &parser.SymbolNode{Location: loc, Unescaped: parser.RubyString{Value: s}}
	}
	rows := make([]parser.Node, len(ps))
	for i, p := range ps {
		row := []parser.Node{sym(p.kind)}
		if p.name != "" {
			row = append(row, sym(p.name))
		}
		rows[i] = &parser.ArrayNode{Location: loc, Elements: row}
	}
	arr := f.c.classes["Array"]
	want := TClass{C: arr, Args: []Type{TClass{C: arr, Args: []Type{f.cls("Symbol")}}}}
	return f.genExpr(f.c.rewrite(n, func() parser.Node { return &parser.ArrayNode{Location: loc, Elements: rows} }), want), true
}

func asBlockParams(n parser.Node) *parser.BlockParametersNode {
	bp, _ := n.(*parser.BlockParametersNode)
	return bp
}
