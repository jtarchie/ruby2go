package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

// csvOptions are the CSV options rb2go implements; any other literal key
// is a compile error rather than silently ignored (decision 117).
var csvOptions = map[string]bool{"col_sep": true, "quote_char": true, "row_sep": true, "skip_blanks": true, "force_quotes": true, "headers": true, "converters": true}

// csvConverters are the converters CSV's literal `converters:` may name.
var csvConverters = map[string]bool{"numeric": true, "integer": true, "float": true}

// csvOverload routes CSV.parse/read/foreach/parse_line by their literal
// options (decision 117): `headers: true` reads a CSV::Table, `converters:`
// gives untyped fields; unknown keys and converters are compile errors.
func (f *fctx) csvOverload(m *Method, owner *Class, args []parser.Node) *entry {
	if len(args) == 0 {
		return nil
	}
	kw, ok := args[len(args)-1].(*parser.KeywordHashNode)
	if !ok {
		return nil
	}
	headers, converters := false, false
	for _, el := range kw.Elements {
		a, ok := el.(*parser.AssocNode)
		if !ok { // `**h`: a dynamic wrapper's keywords, not literal options
			continue
		}
		key, _ := a.Key.(*parser.SymbolNode)
		if key == nil {
			continue
		}
		name := key.Unescaped.Value
		if !csvOptions[name] {
			f.errorf(el, "CSV option %s: is not supported (docs/design.md decision 117)", name)
		}
		switch name {
		case "headers":
			switch a.Value.(type) {
			case *parser.TrueNode:
				headers = true
			case *parser.FalseNode, *parser.NilNode:
			default:
				f.errorf(a.Value, "CSV headers: must be a literal true or false (an Array of names is not supported)")
			}
		case "converters":
			converters = true
			f.csvCheckConverters(a.Value)
		}
	}
	if (headers || converters) && m.Name != "parse" && m.Name != "read" && m.Name != "foreach" && m.Name != "parse_line" {
		f.errorf(kw, "CSV.%s does not take headers: or converters:", m.Name)
	}
	switch {
	case headers && m.Name == "parse_line":
		f.errorf(kw, "CSV.parse_line with headers: is not supported")
	case headers:
		return owner.lookup("__" + m.Name + "_headers")
	case converters:
		return owner.lookup("__" + m.Name + "_converted")
	}
	return nil
}

func (f *fctx) csvCheckConverters(v parser.Node) {
	var names []parser.Node
	if arr, ok := v.(*parser.ArrayNode); ok {
		names = arr.Elements
	} else {
		names = []parser.Node{v}
	}
	for _, n := range names {
		sym, ok := n.(*parser.SymbolNode)
		if !ok || !csvConverters[sym.Unescaped.Value] {
			f.errorf(n, "CSV converters: takes :numeric, :integer or :float (or an Array of them)")
		}
	}
}
