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
    name  string
    value any
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

  func rbConstFind(m rbModule, name string) (any, bool) {
    for _, c := range m._Consts() {
      if c.name == name {
        return c.value, true
      }
    }
    return nil, false
  }

  // rbConstResolve walks an "A::B" path from m; the first segment also
  // falls back to the top level, as Module#const_get does. On a miss it
  // returns the NameError message.
  func rbConstResolve(m rbModule, path string) (any, string) {
    parts := strings.Split(path, "::")
    if parts[0] == "" {
      parts, m = parts[1:], Object_class
    }
    var val any
    for i, p := range parts {
      v, ok := rbConstFind(m, p)
      if !ok && i == 0 {
        v, ok = rbConstFind(Object_class, p)
      }
      if !ok {
        if owner := string(m.Name()); owner != "Object" {
          return nil, "uninitialized constant " + owner + "::" + p
        }
        return nil, "uninitialized constant " + p
      }
      val = v
      if i < len(parts)-1 {
        next, ok := v.(rbModule)
        if !ok {
          panic(NewTypeError(Ref(rbInspect(v) + " is not a class/module")))
        }
        m = next
      }
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
      *out = append(*out, Symbol(c.name))
    }
    return out
  }

  # The transpiler narrows the result type: a literal name gets that
  # constant's type, any other name the join of the module's constants.
  #: (untyped, ?bool) -> untyped
  def const_get(name, inherit = true) = %x{
    val, missing := rbConstResolve(self, rbConstName(name))
    if missing != "" {
      panic(NewNameError(Ref(String(missing))))
    }
    return val
  }

  #: (untyped, ?bool) -> bool
  def const_defined?(name, inherit = true) = %x{
    _, missing := rbConstResolve(self, rbConstName(name))
    return Boolean(missing == "")
  }
end

class Class < Module
end
