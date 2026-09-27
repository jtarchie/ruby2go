# prelude/module.rb
# rbs_inline: enabled
#
# Class objects. Every class and module has one: an instance of its
# metaclass, which the transpiler generates and which inherits from Class
# (or Module, for modules). Each metaclass also gets a generated constant
# table, in definition order; MRI orders `constants` by its symbol table,
# so programs should not depend on the order.

%x{
  type rbConst struct {
    name      string
    value     any
    inherited bool // from a superclass; `inherit = false` skips it
  }

  type rbModule interface {
    _Consts() []rbConst
    _Kind() string // "class" or "module", for messages
    Name() String
  }

  func rbConstName(name any) string {
    switch n := name.(type) {
    case Symbol:
      return string(n)
    case String:
      return string(n)
    }
    panic(NewTypeError(Ref(rbInspect(name) + " is not a symbol nor a string")))
  }

  func rbConstFind(m rbModule, name string, inherit bool) (any, bool) {
    for _, c := range m._Consts() {
      if c.name == name && (inherit || !c.inherited) {
        return c.value, true
      }
    }
    return nil, false
  }

  // MRI's rule: uppercase first letter, then identifier (or any non-ASCII) characters.
  func rbIsConstName(s string) bool {
    for i, r := range s {
      switch {
      case i == 0:
        if !unicode.IsUpper(r) && !unicode.IsTitle(r) {
          return false
        }
      case r < utf8.RuneSelf && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r):
        return false
      }
    }
    return s != ""
  }

  // rbConstResolve walks an "A::B" path from m; the first segment also
  // falls back to the top level, as Module#const_get does. Errors are
  // raised segment by segment, in MRI's order. On a miss it returns the
  // NameError message.
  func rbConstResolve(m rbModule, name any, inherit bool) (any, string) {
    var parts []string
    var path string
    switch n := name.(type) {
    case Symbol:
      path = string(n)
      parts = []string{path}
    case String:
      path = string(n)
      rest := path
      if len(rest) > 2 && strings.HasPrefix(rest, "::") {
        rest, m = rest[2:], Object_class
      }
      parts = strings.Split(rest, "::")
    default:
      what := string(rbInspect(name))
      if _, ok := name.(Boolean); !ok && name != nil {
        what = rbClassName(name)
      }
      panic(NewTypeError(Ref(String("no implicit conversion of " + what + " into String"))))
    }
    var val any
    for i, p := range parts {
      if p == "" || strings.Contains(p, ":") || (i == len(parts)-2 && parts[i+1] == "") {
        panic(NewNameError(Ref(String("wrong constant name " + path))))
      }
      if i > 0 {
        next, ok := val.(rbModule)
        if !ok {
          panic(NewTypeError(Ref(String(path + " does not refer to class/module"))))
        }
        m = next
      }
      if !rbIsConstName(p) {
        panic(NewNameError(Ref(String("wrong constant name " + p))))
      }
      v, ok := rbConstFind(m, p, inherit)
      if !ok && i == 0 && inherit {
        v, ok = rbConstFind(Object_class, p, true)
      }
      if !ok {
        if owner := string(m.Name()); owner != "Object" {
          return nil, "uninitialized constant " + owner + "::" + p
        }
        return nil, "uninitialized constant " + p
      }
      val = v
    }
    return val, ""
  }
}

class Module < Object
  # Overridden by every class object's generated name.
  #: () -> String
  def name = ""

  #: () -> String
  def to_s = name

  #: () -> String
  def inspect = name

  #: (?bool) -> Array[Symbol]
  def constants(inherit = true) = %x{
    out := &Array[Symbol]{}
    for _, c := range self._Consts() {
      if bool(inherit) || !c.inherited {
        *out = append(*out, Symbol(c.name))
      }
    }
    return out
  }

  # The transpiler narrows the result type: a literal name gets that
  # constant's type, any other name the join of the module's constants.
  #: (untyped, ?bool) -> untyped
  def const_get(name, inherit = true) = %x{
    val, missing := rbConstResolve(self, name, bool(inherit))
    if missing != "" {
      panic(NewNameError(Ref(String(missing))))
    }
    return val
  }

  #: (untyped, ?bool) -> bool
  def const_defined?(name, inherit = true) = %x{
    _, missing := rbConstResolve(self, name, bool(inherit))
    return Boolean(missing == "")
  }
end

class Class < Module
end
