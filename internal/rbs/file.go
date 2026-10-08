package rbs

import (
	"errors"
	"fmt"
	"strings"
)

// File is a parsed RBS declaration file: the subset rb2go attaches to
// unannotated Ruby (docs/design.md decision 160). Only declarations are
// modelled; a line rb2go does not understand is skipped, so a gem's sig file
// may name a wider surface than the app reaches.
type File struct {
	Decls []*ClassDecl
}

// ClassDecl is a `class`, `module` or `interface` declaration. Nested
// declarations are also collected in File.Decls, under their qualified Name.
type ClassDecl struct {
	Name        string // qualified with the enclosing declarations: "Outer::Inner"
	IsModule    bool
	IsInterface bool
	TypeParams  []string
	Super       Type // nil unless the class has one
	Includes    []*IncludeDecl
	Methods     []*MethodDecl
	Attrs       []*AttrDecl
	Consts      []*ConstDecl
	Aliases     []*TypeAliasDecl
	Line        int
}

// MethodDecl is `def name: T` or `def self.name: T`, with `#|`-style
// overloads as several entries in Overloads.
type MethodDecl struct {
	Name      string
	Self      bool
	Overloads []*MethodType
	Line      int
}

// AttrDecl is `attr_reader name: T` (Kind "reader", "writer" or "accessor"); Self marks `attr_reader self.name: T`.
type AttrDecl struct {
	Kind string
	Name string
	Self bool
	Type Type
	Line int
}

// ConstDecl is `NAME: T`.
type ConstDecl struct {
	Name string
	Type Type
	Line int
}

// TypeAliasDecl is `type name = T`.
type TypeAliasDecl struct {
	Name string
	Type Type
	Line int
}

// IncludeDecl is `include M`, `extend M` or `prepend M`; Self marks extend.
type IncludeDecl struct {
	Module string
	Args   []Type
	Self   bool
	Line   int
}

// ParseFile parses RBS declaration syntax into the declarations rb2go uses.
func ParseFile(src string) (*File, error) {
	f := &File{}
	var stack []*ClassDecl
	lines := strings.Split(src, "\n")
	for i := range lines {
		line := strings.TrimSpace(stripComment(lines[i]))
		if line == "" {
			continue
		}
		switch {
		case line == "end":
			if len(stack) == 0 {
				return nil, fmt.Errorf("rbs: unmatched `end` on line %d", i+1)
			}
			stack = stack[:len(stack)-1]
		case strings.HasPrefix(line, "class "):
			d, err := parseClass(line, i+1)
			if err != nil {
				return nil, err
			}
			openDecl(f, &stack, d)
		case strings.HasPrefix(line, "module "):
			d, err := parseModule(line, i+1)
			if err != nil {
				return nil, err
			}
			openDecl(f, &stack, d)
		case strings.HasPrefix(line, "interface "):
			d, err := parseInterface(line, i+1)
			if err != nil {
				return nil, err
			}
			openDecl(f, &stack, d)
		default:
			var owner *ClassDecl
			if len(stack) > 0 {
				owner = stack[len(stack)-1]
			}
			start := i + 1
			for strings.HasPrefix(line, "def ") && i+1 < len(lines) {
				nxt := strings.TrimSpace(stripComment(lines[i+1]))
				if !strings.HasPrefix(nxt, "|") {
					break
				}
				line += " " + nxt
				i++
			}
			err := parseMember(owner, line, start)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("rbs: %s is never closed", stack[len(stack)-1].Name)
	}
	return f, nil
}

func openDecl(f *File, stack *[]*ClassDecl, d *ClassDecl) {
	if n := len(*stack); n > 0 {
		d.Name = (*stack)[n-1].Name + "::" + d.Name
	}
	f.Decls = append(f.Decls, d)
	*stack = append(*stack, d)
}

func parseClass(line string, ln int) (*ClassDecl, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "class"))
	name, tps, rest, err := parseHead(rest)
	if err != nil {
		return nil, fmt.Errorf("rbs: class on line %d: %w", ln, err)
	}
	d := &ClassDecl{Name: name, TypeParams: tps, Line: ln}
	if strings.HasPrefix(rest, "<") {
		super, err := ParseType(strings.TrimSpace(rest[1:]))
		if err != nil {
			return nil, fmt.Errorf("rbs: class %s superclass on line %d: %w", name, ln, err)
		}
		d.Super = super
	}
	return d, nil
}

func parseModule(line string, ln int) (*ClassDecl, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "module"))
	name, tps, _, err := parseHead(rest)
	if err != nil {
		return nil, fmt.Errorf("rbs: module on line %d: %w", ln, err)
	}
	return &ClassDecl{Name: name, IsModule: true, TypeParams: tps, Line: ln}, nil
}

func parseInterface(line string, ln int) (*ClassDecl, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "interface"))
	name, tps, _, err := parseHead(rest)
	if err != nil {
		return nil, fmt.Errorf("rbs: interface on line %d: %w", ln, err)
	}
	return &ClassDecl{Name: name, IsModule: true, IsInterface: true, TypeParams: tps, Line: ln}, nil
}

// parseHead reads a declaration header after its keyword: the qualified name,
// optional `[type params]`, and whatever follows (a `< superclass` or a `:`
// module self-type). Type params keep only their names, dropping variance and
// bounds, as the `# @rbs generic` annotation does.
func parseHead(rest string) (name string, tps []string, tail string, err error) {
	rest = strings.TrimSpace(rest)
	i := 0
	for i < len(rest) {
		if rest[i] == ':' && i+1 < len(rest) && rest[i+1] == ':' {
			i += 2
			continue
		}
		if rest[i] == '[' || rest[i] == '<' || rest[i] == ' ' || rest[i] == ':' {
			break
		}
		i++
	}
	name = rest[:i]
	if name == "" {
		return "", nil, "", errors.New("expected a name")
	}
	rest = strings.TrimSpace(rest[i:])
	if strings.HasPrefix(rest, "[") {
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			return "", nil, "", fmt.Errorf("unterminated type parameters in %q", name)
		}
		tps = parseTypeParams(rest[1:end])
		rest = strings.TrimSpace(rest[end+1:])
	}
	return name, tps, rest, nil
}

func parseTypeParams(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if i := strings.IndexByte(part, '<'); i >= 0 {
			part = strings.TrimSpace(part[:i])
		}
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		out = append(out, fields[len(fields)-1])
	}
	return out
}

func parseMember(owner *ClassDecl, line string, ln int) error {
	switch {
	case strings.HasPrefix(line, "def "):
		m, err := parseMethod(strings.TrimSpace(strings.TrimPrefix(line, "def")), ln)
		if err != nil {
			return err
		}
		if owner != nil {
			owner.Methods = append(owner.Methods, m)
		}
	case strings.HasPrefix(line, "attr_reader "):
		return addAttr(owner, "reader", line, ln)
	case strings.HasPrefix(line, "attr_writer "):
		return addAttr(owner, "writer", line, ln)
	case strings.HasPrefix(line, "attr_accessor "):
		return addAttr(owner, "accessor", line, ln)
	case strings.HasPrefix(line, "include "), strings.HasPrefix(line, "extend "), strings.HasPrefix(line, "prepend "):
		if owner != nil {
			inc, err := parseInclude(line, ln)
			if err != nil {
				return err
			}
			owner.Includes = append(owner.Includes, inc)
		}
	case strings.HasPrefix(line, "type "):
		if owner != nil {
			return addAlias(owner, line, ln)
		}
	case line == "private" || line == "public":
		// visibility is not modelled
	default:
		if owner != nil {
			return addConst(owner, line, ln)
		}
	}
	return nil
}

func parseMethod(s string, ln int) (*MethodDecl, error) {
	name, sigText, ok := splitMember(s)
	if !ok {
		return nil, fmt.Errorf("rbs: def on line %d: expected `name: type`", ln)
	}
	m := &MethodDecl{Name: name, Line: ln}
	if strings.HasPrefix(name, "self.") {
		m.Self, m.Name = true, strings.TrimPrefix(name, "self.")
	} else if strings.HasPrefix(name, "self?.") {
		m.Self, m.Name = true, strings.TrimPrefix(name, "self?.")
	}
	overloads, err := ParseMethodTypes(sigText)
	if err != nil {
		return nil, fmt.Errorf("rbs: def %s on line %d: %w", m.Name, ln, err)
	}
	m.Overloads = overloads
	return m, nil
}

func addAttr(owner *ClassDecl, kind, line string, ln int) error {
	if owner == nil {
		return nil
	}
	rest := strings.TrimSpace(line[strings.IndexByte(line, ' ')+1:])
	name, typeText, ok := splitMember(rest)
	if !ok {
		return fmt.Errorf("rbs: attr_%s on line %d: expected `name: type`", kind, ln)
	}
	t, err := ParseType(typeText)
	if err != nil {
		return fmt.Errorf("rbs: attr_%s %s on line %d: %w", kind, name, ln, err)
	}
	name, self := strings.CutPrefix(name, "self.")
	owner.Attrs = append(owner.Attrs, &AttrDecl{Kind: kind, Name: name, Self: self, Type: t, Line: ln})
	return nil
}

func addConst(owner *ClassDecl, line string, ln int) error {
	name, typeText, ok := splitMember(line)
	if !ok || name == "" || !isConstName(name) {
		return nil // not a declaration rb2go models
	}
	t, err := ParseType(typeText)
	if err != nil {
		return fmt.Errorf("rbs: constant %s on line %d: %w", name, ln, err)
	}
	owner.Consts = append(owner.Consts, &ConstDecl{Name: name, Type: t, Line: ln})
	return nil
}

func addAlias(owner *ClassDecl, line string, ln int) error {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "type"))
	name, typeText, ok := strings.Cut(rest, "=")
	if !ok {
		return fmt.Errorf("rbs: type alias on line %d: expected `name = type`", ln)
	}
	t, err := ParseType(typeText)
	if err != nil {
		return fmt.Errorf("rbs: type %s on line %d: %w", strings.TrimSpace(name), ln, err)
	}
	owner.Aliases = append(owner.Aliases, &TypeAliasDecl{Name: strings.TrimSpace(name), Type: t, Line: ln})
	return nil
}

func parseInclude(line string, ln int) (*IncludeDecl, error) {
	self := strings.HasPrefix(line, "extend ")
	rest := strings.TrimSpace(line[strings.IndexByte(line, ' ')+1:])
	inc := &IncludeDecl{Self: self, Line: ln}
	name, _, _, err := parseHead(rest)
	if err != nil {
		return nil, fmt.Errorf("rbs: include on line %d: %w", ln, err)
	}
	inc.Module = name
	if strings.HasPrefix(strings.TrimSpace(rest), "[") {
		t, err := ParseType(name + strings.TrimSpace(rest[len(name):]))
		if err != nil {
			return nil, fmt.Errorf("rbs: include %s on line %d: %w", name, ln, err)
		}
		if n, ok := t.(Name); ok {
			inc.Args = n.Args
		}
	}
	return inc, nil
}

// splitMember splits `name: type` at the first colon that is neither `::` nor
// a keyword parameter's. type text is trimmed.
func splitMember(s string) (name, rest string, ok bool) {
	for i := range len(s) {
		if s[i] != ':' {
			continue
		}
		if i+1 < len(s) && s[i+1] == ':' || i > 0 && s[i-1] == ':' {
			continue
		}
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
	}
	return "", "", false
}

func isConstName(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == ':' {
			continue
		}
		return false
	}
	return true
}

// stripComment removes a `#` comment. A `#` inside a string-literal type
// (decision 12's literal types) would be misread; literal types are rejected
// by the type parser for now.
func stripComment(s string) string {
	if i := strings.IndexByte(s, '#'); i >= 0 {
		return s[:i]
	}
	return s
}

// ParseMethodTypes parses one method type, or several overloads separated by a
// top-level `|` (`(A) -> X | (B) -> Y`). A `|` that is a union in the return
// type stays inside one method type: a part that is not itself a method type
// is folded back into the previous arm's return.
func ParseMethodTypes(s string) ([]*MethodType, error) {
	var out []*MethodType
	cur := ""
	for _, part := range splitOverloads(s) {
		_, err := ParseMethodType(part)
		switch {
		case err == nil && cur != "":
			prev, perr := ParseMethodType(cur)
			if perr != nil {
				return nil, perr
			}
			out = append(out, prev)
			cur = part
		case err == nil:
			cur = part
		case cur == "":
			cur = part
		default:
			cur += " | " + part
		}
	}
	m, err := ParseMethodType(cur)
	if err != nil {
		return nil, err
	}
	return append(out, m), nil
}

// splitOverloads splits on `|` at brace/paren/bracket depth 0.
func splitOverloads(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := range len(s) {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '|':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out
}
