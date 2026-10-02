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

  # Case equality: obj is an instance of this class (or includes this
  # module). The class object's generated _IsInstance answers (decision 76).
  #: (untyped) -> bool
  def ===(obj) = %x{ Boolean(rbIsInstanceOf(self, obj)) }

  #: (?bool) -> Array[Symbol]
  def constants(inherit = true) = %x{
    out := &Array[Symbol]{}
    for _, c := range any(self).(rbConstTable)._Consts() {
      if bool(inherit) || !c.inherited {
        *out = append(*out, Symbol(c.name))
      }
    }
    return out
  }

  # Public instance methods from a generated table, without Object's and
  # Kernel's (decision 77). instance_methods is the same: there is no
  # protected.
  #: (?bool) -> Array[Symbol]
  def public_instance_methods(inherit = true) = %x{
    out := &Array[Symbol]{}
    for _, m := range any(self).(interface{ _Methods() []rbConst })._Methods() {
      if bool(inherit) || !m.inherited {
        *out = append(*out, Symbol(m.name))
      }
    }
    return out
  }

  #: (?bool) -> Array[Symbol]
  def instance_methods(inherit = true) = public_instance_methods(inherit)

  #: (untyped, ?bool) -> bool
  def public_method_defined?(name, inherit = true) = public_instance_methods(inherit).include?(name.to_s.to_sym)

  #: (untyped, ?bool) -> bool
  def method_defined?(name, inherit = true) = public_method_defined?(name, inherit)

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
  # The parent's class object; nil above Object, whose BasicObject has none in rb2go.
  #: () -> Class?
  def superclass = %x{
    t, ok := any(self).(interface{ _Superclass() any })
    if !ok {
      return nil
    }
    k, ok := t._Superclass().(ClassI)
    if !ok {
      return nil
    }
    return &k
  }

  #: () -> Array[Class]
  def subclasses = %x{
    out := &Array[ClassI]{}
    if t, ok := any(self).(interface{ _Subclasses() []any }); ok {
      for _, k := range t._Subclasses() {
        *out = append(*out, k.(ClassI))
      }
    }
    return out
  }
end
