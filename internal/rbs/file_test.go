package rbs

import "testing"

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
	if len(app.Attrs) != 3 || app.Attrs[0].Kind != "reader" || app.Attrs[1].Kind != "writer" || app.Attrs[2].Kind != "accessor" {
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
