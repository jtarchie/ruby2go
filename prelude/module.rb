# prelude/module.rb
# rbs_inline: enabled
#
# Class objects. Every class and module has one: an instance of its
# metaclass, which the transpiler generates and which inherits from Class
# (or Module, for modules). Each metaclass also gets a generated constant
# table, in definition order; MRI orders `constants` by its symbol table,
# so programs should not depend on the order.

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
