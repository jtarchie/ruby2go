package rbs

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// File is a parsed RBS declaration file: the subset rb2go attaches to
// unannotated Ruby (docs/design.md decision 160). Only declarations are
// modelled; a line rb2go does not understand is skipped, so a gem's sig file
// may name a wider surface than the app reaches. Top-level `type` aliases and constants are dropped on purpose: File has no slot, nothing consumes them.
type File struct {
	Decls []*ClassDecl
}

// ClassDecl is a `class`, `module` or `interface` declaration. Nested
// declarations are also collected in File.Decls, under their qualified Name.
type ClassDecl struct {
	Name          string // qualified with the enclosing declarations: "Outer::Inner"
	IsModule      bool
	IsInterface   bool
	TypeParams    []string
	Bounds        map[string]string // `[P < Comparable[P]]`: each bounded parameter's bound class name (decision 312)
	Super         Type              // nil unless the class has one
	Includes      []*IncludeDecl
	Methods       []*MethodDecl
	Attrs         []*AttrDecl
	Ivars         []*ConstDecl // `@x: T`, and `self.@x: T` for the class object's (Name keeps the prefix)
	Consts        []*ConstDecl
	Aliases       []*TypeAliasDecl
	MethodAliases []*MethodAliasDecl
	Line          int
}

// MethodAliasDecl is `alias new old` (Self: `alias self.new self.old`).
type MethodAliasDecl struct {
	New, Old string
	Self     bool
	Line     int
}

// MethodDecl is `def name: T` or `def self.name: T`, with `#|`-style
// overloads as several entries in Overloads. `def self?.name: T` (a
// module_function) sets both Self and Both.
type MethodDecl struct {
	Name      string
	Self      bool
	Both      bool // `self?.`: the singleton method and the instance method
	Overloads []*MethodType
	Line      int
}

// AttrKind is which accessor an AttrDecl declares.
type AttrKind string

const (
	AttrReader   AttrKind = "reader"
	AttrWriter   AttrKind = "writer"
	AttrAccessor AttrKind = "accessor"
)

// AttrDecl is `attr_reader name: T` (Kind AttrReader, AttrWriter or AttrAccessor); Self marks `attr_reader self.name: T`.
type AttrDecl struct {
	Kind AttrKind
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

// IncludeKind is the keyword of an IncludeDecl.
type IncludeKind string

const (
	IncludeInclude IncludeKind = "include"
	IncludeExtend  IncludeKind = "extend"
	IncludePrepend IncludeKind = "prepend"
)

// IncludeDecl is `include M`, `extend M` or `prepend M`; Self marks extend, Kind is the keyword.
type IncludeDecl struct {
	Kind   IncludeKind
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
		case classAlias.MatchString(line):
			// a class alias (`class Array = ArrayType`) opens no body
		case strings.HasPrefix(line, "class "):
			err := openDecl(f, &stack, parseClass, line, i+1)
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, "module "):
			err := openDecl(f, &stack, parseModule, line, i+1)
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, "interface "):
			err := openDecl(f, &stack, parseInterface, line, i+1)
			if err != nil {
				return nil, err
			}
		default:
			var owner *ClassDecl
			if len(stack) > 0 {
				owner = stack[len(stack)-1]
			}
			start := i + 1
			line = joinMember(lines, &i, line)
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

// joinMember joins a member's continuation lines (open bracket, leading `|`), dropping `%a{}`; depth is carried, not rescanned, so an unclosed `(` stays O(N).
func joinMember(lines []string, i *int, line string) string {
	var sb strings.Builder
	sb.WriteString(line)
	_, depth := scanCode(line)
	endsPipe := strings.HasSuffix(line, "|")
	for *i+1 < len(lines) {
		nxt := strings.TrimSpace(stripComment(lines[*i+1]))
		if !strings.HasPrefix(nxt, "|") && !endsPipe && depth <= 0 {
			break
		}
		sb.WriteString(" " + nxt)
		_, d := scanCode(nxt)
		depth += d
		endsPipe = strings.HasSuffix(nxt, "|")
		*i++
	}
	return strings.TrimSpace(annotation.ReplaceAllString(sb.String(), ""))
}

var classAlias = regexp.MustCompile(`^(class|module) [\w:]+ =`)

// annotation is an RBS `%a{...}`, `%a(...)` or `%a[...]` annotation, which rb2go does not model.
var annotation = regexp.MustCompile(`%a(\{[^}]*\}|\([^)]*\)|\[[^\]]*\])`)

// scanCode returns the first `#` outside a string literal (len(s) if none) and the bracket depth before it; quoted brackets do not count.
func scanCode(s string) (end, depth int) {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '#':
			return i, depth
		case '"', '\'':
			for i++; i < len(s) && s[i] != c; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		}
	}
	return len(s), depth
}

func openDecl(f *File, stack *[]*ClassDecl, parse func(string, int) (*ClassDecl, error), line string, ln int) error {
	if len(*stack) >= maxDepth {
		return errTooDeep
	}
	d, err := parse(line, ln)
	if err != nil {
		return err
	}
	if n := len(*stack); n > 0 {
		d.Name = (*stack)[n-1].Name + "::" + d.Name
	}
	f.Decls = append(f.Decls, d)
	*stack = append(*stack, d)
	return nil
}

func parseClass(line string, ln int) (*ClassDecl, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "class"))
	name, tps, rest, err := parseHead(rest)
	if err != nil {
		return nil, fmt.Errorf("rbs: class on line %d: %w", ln, err)
	}
	d := &ClassDecl{Name: name, TypeParams: tps, Line: ln, Bounds: typeBounds(line)}
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
		end := closeBracket(rest)
		if end < 0 {
			return "", nil, "", fmt.Errorf("unterminated type parameters in %q", name)
		}
		tps = parseTypeParams(rest[1:end])
		rest = strings.TrimSpace(rest[end+1:])
	}
	return name, tps, rest, nil
}

// closeBracket is the index of the `]` closing s's leading `[`, or -1: a bound may nest (`[P < Comparable[P]]`).
func closeBracket(s string) int {
	depth := 0
	for i := range len(s) {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// typeBounds reads a class line's `[E, P < Comparable[P]]`: the bound's name per parameter, its arguments dropped.
func typeBounds(line string) map[string]string {
	i := strings.IndexByte(line, '[')
	if i < 0 || strings.IndexByte(line, '<') >= 0 && strings.IndexByte(line, '<') < i { // `class X < Base[E]`: no params of its own
		return nil
	}
	end := closeBracket(line[i:])
	if end < 0 {
		return nil
	}
	var out map[string]string
	depth, start := 0, i+1
	for j := i + 1; j <= i+end; j++ {
		switch {
		case line[j] == '[':
			depth++
		case line[j] == ']' && depth > 0:
			depth--
		case depth == 0 && (line[j] == ',' || j == i+end):
			if name, bound, ok := strings.Cut(line[start:j], "<"); ok {
				fields := strings.Fields(name)
				b, _, _ := strings.Cut(strings.TrimSpace(bound), "[")
				if len(fields) > 0 && b != "" {
					if out == nil {
						out = map[string]string{}
					}
					out[fields[len(fields)-1]] = strings.TrimSpace(b)
				}
			}
			start = j + 1
		}
	}
	return out
}

func parseTypeParams(s string) []string {
	var top strings.Builder // s without a bound's brackets, whose commas are not separators
	depth := 0
	for i := range len(s) {
		switch {
		case s[i] == '[':
			depth++
		case s[i] == ']':
			depth--
		case depth == 0:
			top.WriteByte(s[i])
		}
	}
	var out []string
	for _, part := range strings.Split(top.String(), ",") {
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
	// visibility is not modelled: `private def x: ...` is `def x: ...`
	if rest, ok := strings.CutPrefix(line, "private "); ok {
		line = strings.TrimSpace(rest)
	} else if rest, ok := strings.CutPrefix(line, "public "); ok {
		line = strings.TrimSpace(rest)
	}
	switch {
	case strings.HasPrefix(line, "def "):
		m, err := parseMethod(strings.TrimSpace(strings.TrimPrefix(line, "def")), ln)
		if err != nil {
			return err
		}
		if owner != nil && len(m.Overloads) > 0 {
			owner.Methods = append(owner.Methods, m)
		}
	case strings.HasPrefix(line, "@"), strings.HasPrefix(line, "self.@"):
		return addIvar(owner, line, ln)
	case strings.HasPrefix(line, "attr_reader "):
		return addAttr(owner, AttrReader, line, ln)
	case strings.HasPrefix(line, "attr_writer "):
		return addAttr(owner, AttrWriter, line, ln)
	case strings.HasPrefix(line, "attr_accessor "):
		return addAttr(owner, AttrAccessor, line, ln)
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
	case strings.HasPrefix(line, "alias "):
		if f := strings.Fields(line); owner != nil && len(f) == 3 {
			nw, self := strings.CutPrefix(f[1], "self.")
			owner.MethodAliases = append(owner.MethodAliases, &MethodAliasDecl{New: nw, Old: strings.TrimPrefix(f[2], "self."), Self: self, Line: ln})
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
		m.Self, m.Both, m.Name = true, true, strings.TrimPrefix(name, "self?.")
	}
	overloads, err := ParseMethodTypes(sigText)
	if err != nil {
		return nil, fmt.Errorf("rbs: def %s on line %d: %w", m.Name, ln, err)
	}
	m.Overloads = overloads
	return m, nil
}

func addAttr(owner *ClassDecl, kind AttrKind, line string, ln int) error {
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

func addIvar(owner *ClassDecl, line string, ln int) error {
	name, typeText, ok := splitMember(line)
	if !ok {
		return fmt.Errorf("rbs: instance variable on line %d: expected `@name: type`", ln)
	}
	t, err := ParseType(typeText)
	if err != nil {
		return fmt.Errorf("rbs: %s on line %d: %w", name, ln, err)
	}
	if owner != nil {
		owner.Ivars = append(owner.Ivars, &ConstDecl{Name: name, Type: t, Line: ln})
	}
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
	inc := &IncludeDecl{Kind: IncludeKind(line[:strings.IndexByte(line, ' ')]), Self: self, Line: ln}
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

// stripComment removes a `#` comment; a `#` inside a string-literal type (`-> "#"`, a record key `"#" =>`) is kept.
func stripComment(s string) string {
	end, _ := scanCode(s)
	return s[:end]
}

// ParseMethodTypes parses one method type, or several overloads separated by a
// top-level `|` (`(A) -> X | (B) -> Y`). A `|` that is a union in the return
// type stays inside one method type: a part that is not itself a method type
// is folded back into the previous arm's return.
func ParseMethodTypes(s string) ([]*MethodType, error) {
	var out []*MethodType
	cur := ""
	var curM *MethodType // cur's parse, when cur is one whole part (a folded union return is parsed once at the end)
	for _, part := range splitOverloads(s) {
		if part == "..." { // the overloads the class already has: nothing for rb2go to add
			continue
		}
		pm, err := ParseMethodType(part)
		switch {
		case err == nil && cur != "":
			prev, perr := parsedOr(curM, cur)
			if perr != nil {
				return nil, perr
			}
			out = append(out, prev)
			cur, curM = part, pm
		case err == nil:
			cur, curM = part, pm
		case cur == "":
			cur, curM = part, nil
		default:
			cur, curM = cur+" | "+part, nil
		}
	}
	if cur == "" && len(out) == 0 {
		return nil, nil // `def x: ...`
	}
	m, err := parsedOr(curM, cur)
	if err != nil {
		return nil, err
	}
	return append(out, m), nil
}

// parsedOr is m when the part was already parsed whole, else the parse of s (a union return folded back into its arm).
func parsedOr(m *MethodType, s string) (*MethodType, error) {
	if m != nil {
		return m, nil
	}
	return ParseMethodType(s)
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
