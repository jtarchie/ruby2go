# prelude/struct.rb
# rbs_inline: enabled
#
# The superclass of every `Name = Struct.new(:a, :b)` class. The transpiler
# generates each struct's accessors, initialize, ==, eql?, hash, to_a,
# members and inspect; the member types come from a trailing `#: [A, B]`.

class Struct < Object
  # A writer checks the object's own flag, as MRI's rb_check_frozen does, not a frozen? override; with no freeze in the program it is false at compile time (decision 385).
  #: () -> bool
  def __frozen_flag? = %x{
    p := rbObjFrozenFlag(any(self))
    return Boolean(p != nil && *p)
  }
end

# The superclass of every `Name = Data.define(:a, :b)` class: immutable
# values with readers, keyword or positional `new` (every member required),
# `with`, `to_h`, ==, eql?, hash, members and inspect, all generated.
class Data < Object
  #: () -> bool
  def frozen? = true
end
