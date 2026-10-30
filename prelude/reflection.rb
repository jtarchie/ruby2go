# rbs_inline: enabled

# Loaded when a file calls private_methods or protected_methods (decision 416): as Kernel methods every object answers
# them, and a program with a computed send would compile every class's method tables for them.
module Kernel
  # The private and protected methods its class defines (drb's check_insecure_method refuses them, decision 416).
  # ponytail: not a singleton class's own, nor Kernel's private methods MRI lists too
  #: (?bool) -> Array[Symbol]
  def private_methods(all = true) = __private_singleton || self.class.private_instance_methods(all)

  # A class object's private class methods (`private_class_method :new`); nil for any other object.
  #: () -> Array[Symbol]?
  def __private_singleton = %x{
    t, ok := any(self).(interface{ _PrivateSingletonMethods() []rbConst })
    if !ok {
      return nil
    }
    out := &Array[Symbol]{}
    for _, m := range t._PrivateSingletonMethods() {
      out.s = append(out.s, Symbol(m.name))
    }
    return &out
  }

  #: (?bool) -> Array[Symbol]
  def protected_methods(all = true) = self.class.protected_instance_methods(all)
end
