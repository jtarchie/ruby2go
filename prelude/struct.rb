# prelude/struct.rb
# rbs_inline: enabled

#
# The superclass of every `Name = Struct.new(:a, :b)` class. The transpiler
# generates each struct's accessors, initialize, ==, eql?, hash, to_a,
# members and inspect; the member types come from a trailing `#: [A, B]`.

class Struct < Object
  # A writer checks the object's own flag, as MRI's rb_check_frozen does, not a frozen? override; with no freeze in the program it is false at compile time (decision 387).
  #: () -> bool
  def __frozen_flag? = %x{
    p := rbObjFrozenFlag(any(self))
    return Boolean(p != nil && *p)
  }

  # One member of a generated Struct#==. Scalars compare directly; anything else (a Struct, Array or Hash that may lead
  # back here) is bracketed by MRI's rb_exec_recursive_paired, so a struct that holds itself answers "equal so far"
  # instead of overflowing the stack (decision 407). A member typed Integer or String is a scalar at compile time,
  # so it pays nothing.
  # true when this goroutine is already comparing (kind, a, b) (0 is ==, 1 is eql?); otherwise it counts the comparison in
  # until __rec_leave.
  #: (Integer, untyped, untyped) -> bool
  def self.__rec_enter(kind, a, b) = %x{ return Boolean(rbRecEnter(kind, a, b)) }

  #: () -> nil
  def self.__rec_leave = %x{ rbRecLeave() }

  # hash of a struct that reaches itself is one constant for the whole call, as MRI's rb_exec_recursive_outer.
  #: (untyped) { () -> Integer } -> Integer
  def self.__hash_rec(obj) = %x{ return rbRecHash(obj, func() Integer { return blk() }) }
end

# The superclass of every `Name = Data.define(:a, :b)` class: immutable
# values with readers, keyword or positional `new` (every member required),
# `with`, `to_h`, ==, eql?, hash, members and inspect, all generated.
class Data < Object
  #: () -> bool
  def frozen? = true
end

# The comparison generated Struct#== uses for a member that could lead back to the struct (decision 407). A module
# function, so a call is a plain Go function: scalars compare directly, anything else is bracketed by MRI's
# rb_exec_recursive_paired, so a struct that holds itself answers "equal so far" instead of overflowing the stack.
module StructEq
  #: (untyped, untyped) -> bool
  def self.same(a, b) = %x{ return Boolean(rbRecSame(a, b)) }
end
