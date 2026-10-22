package rbs

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseFileNestedBoundAndMethodAlias(t *testing.T) {
	src := `class Heap[Elem, Priority]
  def push: (Elem, ?Priority) -> self
  alias enqueue push
  alias self.make self.new
end
class MinQ[Elem, Priority < Comparable[Priority], K < Hash[Elem, Priority]] < Heap[Elem, Priority]
  alias decrease_key change_priority
  %a{pure} def wide: (
    name: String name,
    ?Integer
  ) -> void
  def freeze: ...
  def each: [P < Integer] { (Elem) -> void } -> void
          | ...
  type kind = :a |
              :b
end
class Array = ArrayType
`
	f, err := ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	heap, minq := f.Decls[0], f.Decls[1]
	if len(heap.MethodAliases) != 2 || *heap.MethodAliases[0] != (MethodAliasDecl{New: "enqueue", Old: "push", Line: 3}) ||
		*heap.MethodAliases[1] != (MethodAliasDecl{New: "make", Old: "new", Self: true, Line: 4}) {
		t.Errorf("aliases = %+v", heap.MethodAliases)
	}
	if got := fmt.Sprint(minq.Bounds, heap.Bounds); got != "map[K:Hash Priority:Comparable] map[]" {
		t.Errorf("bounds = %s", got)
	}
	if got := fmt.Sprint(minq.TypeParams); got != "[Elem Priority K]" {
		t.Errorf("type params = %s", got)
	}
	if minq.Super == nil || minq.Super.String() != "Heap[Elem, Priority]" {
		t.Errorf("super = %v", minq.Super)
	}
	if len(minq.MethodAliases) != 1 || minq.MethodAliases[0].New != "decrease_key" {
		t.Errorf("aliases = %+v", minq.MethodAliases)
	}
	if got := fmt.Sprint(minq.Methods[0].Overloads, minq.Methods[1].Overloads, minq.Aliases[0].Type); len(minq.Methods) != 2 ||
		got != "[(name: String, ?Integer) -> void] [[P] () { (Elem) -> void } -> void] :a | :b" {
		t.Errorf("methods = %s", got)
	}
}

func TestParseFileDecls(t *testing.T) {
	src := `# line comment
module Rack
  module Utils
    def escape: (String) -> String
  end
end

class Cuba::App < Object
  include Rack::Utils
  extend Forwardable

  VERSION: String
  LIMIT: Integer

  type Handler = ^(untyped) -> void

  attr_reader env: Hash[String, untyped]
  attr_writer body: String
  attr_accessor code: Integer

  def call: (Hash[String, untyped]) -> Array[untyped]
  def self?.helper: () -> void
end
`
	f, err := ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*ClassDecl{}
	for _, d := range f.Decls {
		byName[d.Name] = d
	}
	for _, want := range []string{"Rack", "Rack::Utils", "Cuba::App"} {
		if byName[want] == nil {
			t.Fatalf("missing declaration %q; got %v", want, keys(byName))
		}
	}
	utils := byName["Rack::Utils"]
	if !utils.IsModule || len(utils.Methods) != 1 || utils.Methods[0].Name != "escape" || utils.Methods[0].Self {
		t.Errorf("Rack::Utils = %+v", utils)
	}
	app := byName["Cuba::App"]
	if app.Super == nil || app.Super.String() != "Object" {
		t.Errorf("super = %v", app.Super)
	}
	if len(app.Includes) != 2 || app.Includes[0].Module != "Rack::Utils" || app.Includes[0].Self || !app.Includes[1].Self {
		t.Errorf("includes = %+v", app.Includes)
	}
	if len(app.Consts) != 2 || app.Consts[1].Name != "LIMIT" || app.Consts[1].Type.String() != "Integer" {
		t.Errorf("consts = %+v", app.Consts)
	}
	if len(app.Aliases) != 1 || app.Aliases[0].Name != "Handler" || app.Aliases[0].Type.String() != "^(untyped) -> void" {
		t.Errorf("aliases = %+v", app.Aliases)
	}
	if len(app.Attrs) != 3 || app.Attrs[0].Kind != AttrReader || app.Attrs[1].Kind != AttrWriter || app.Attrs[2].Kind != AttrAccessor {
		t.Errorf("attrs = %+v", app.Attrs)
	}
	if len(app.Methods) != 2 {
		t.Fatalf("methods = %+v", app.Methods)
	}
	if app.Methods[0].Name != "call" || app.Methods[0].Self {
		t.Errorf("call = %+v", app.Methods[0])
	}
	if app.Methods[1].Name != "helper" || !app.Methods[1].Self {
		t.Errorf("self?.helper = %+v", app.Methods[1])
	}
}

func TestParseFileGenericsAndOverloads(t *testing.T) {
	src := `class Box[out E, unchecked T < Object]
  def []: (Integer) -> E?
  def get: () -> E | nil
          | (Integer) -> E?
end
`
	f, err := ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	box := f.Decls[0]
	if len(box.TypeParams) != 2 || box.TypeParams[0] != "E" || box.TypeParams[1] != "T" {
		t.Errorf("type params = %v", box.TypeParams)
	}
	if len(box.Methods) != 2 {
		t.Fatalf("methods = %+v", box.Methods)
	}
	if len(box.Methods[1].Overloads) != 2 {
		t.Errorf("get overloads = %+v", box.Methods[1].Overloads)
	}
}

func TestParseFileErrors(t *testing.T) {
	cases := []string{
		"class Foo\n",
		"end\n",
		"class Foo\n  def x: Integer\nend\n",
	}
	for _, src := range cases {
		_, err := ParseFile(src)
		if err == nil {
			t.Errorf("ParseFile(%q) succeeded, want error", src)
		}
	}
}

func keys(m map[string]*ClassDecl) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestParseFileIvarsSelfAttrsAndSelfProcs(t *testing.T) {
	src := `module Utils
  @cache: Hash[String, Integer]?
  self.@registry: Array[String]
  attr_accessor self.limit: Integer
  def self?.escape: (String) -> String
  @map: Hash[String, ^() [self: Builder] -> void]
end
`
	f, err := ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	u := f.Decls[0]
	if len(u.Ivars) != 3 || u.Ivars[0].Name != "@cache" || u.Ivars[1].Name != "self.@registry" {
		t.Fatalf("ivars = %+v", u.Ivars)
	}
	if got := u.Ivars[2].Type.String(); got != "Hash[String, ^() [self: Builder] -> void]" {
		t.Errorf("proc with self = %s", got)
	}
	if len(u.Attrs) != 1 || !u.Attrs[0].Self || u.Attrs[0].Name != "limit" {
		t.Errorf("attrs = %+v", u.Attrs)
	}
	if m := u.Methods[0]; !m.Self || !m.Both || m.Name != "escape" {
		t.Errorf("self? method = %+v", m)
	}
}

// TestParseFilePrivateMembers: a `private`/`public` prefix on a member (core/kernel.rbs, prism's dispatcher.rbs) does not hide it.
func TestParseFilePrivateMembers(t *testing.T) {
	src := `class C
  private def respond_to_missing?: (Symbol, bool) -> bool
  public def self.make: () -> C
  private attr_reader secret: String
  private
  def after: () -> void
end
`
	f, err := ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	c := f.Decls[0]
	names := make([]string, 0, len(c.Methods))
	for _, m := range c.Methods {
		names = append(names, fmt.Sprint(m.Name, m.Self))
	}
	if got := fmt.Sprint(names); got != "[respond_to_missing?false maketrue afterfalse]" {
		t.Errorf("methods = %s", got)
	}
	if len(c.Attrs) != 1 || c.Attrs[0].Name != "secret" || len(c.Consts) != 0 {
		t.Errorf("attrs = %+v consts = %+v", c.Attrs, c.Consts)
	}
}

// TestParseFileQuotedHash: a `#` inside a string-literal type is not a comment.
func TestParseFileQuotedHash(t *testing.T) {
	src := "class C\n" +
		"  def sep: () -> \"#\" # trailing comment\n" +
		"  def rec: (a: { \"#\" => Integer, 'k#' => String }) -> void # c\n" +
		"  def open: (\"(\") -> void\n" +
		"  def after: () -> void\n" +
		"end\n"
	f, err := ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	c := f.Decls[0]
	if len(c.Methods) != 4 {
		t.Fatalf("methods = %+v", c.Methods)
	}
	if got := c.Methods[0].Overloads[0].String(); got != `() -> "#"` {
		t.Errorf("sep = %s", got)
	}
	if got := c.Methods[1].Overloads[0].String(); !strings.Contains(got, `"#" => Integer`) {
		t.Errorf("rec = %s", got)
	}
	if c.Methods[2].Name != "open" || c.Methods[2].Overloads[0].String() != `("(") -> void` {
		t.Errorf("open = %+v", c.Methods[2])
	}
}
