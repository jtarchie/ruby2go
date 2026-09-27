# prelude/struct.rb
# rbs_inline: enabled
#
# The superclass of every `Name = Struct.new(:a, :b)` class. The transpiler
# generates each struct's accessors, initialize, ==, to_a, members and
# inspect; the member types come from a trailing `#: [A, B]`.

class Struct < Object
end

# The superclass of every `Name = Data.define(:a, :b)` class: immutable
# values with readers, keyword or positional `new` (every member required),
# `with`, `to_h`, ==, members and inspect, all generated.
class Data < Object
  #: () -> bool
  def frozen? = true
end
