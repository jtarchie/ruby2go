# rbs_inline: enabled

class Temp
  attr_reader :deg #: Integer

  #: (Integer) -> void
  def initialize(deg)
    @deg = deg
  end

  #: () -> String
  def to_s = "#{deg}°"

  #: () -> String
  def inspect = "#<Temp #{deg}>"
end

#: (untyped) -> untyped
def ident(v) = v

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

puts "-- puts of nil prints an empty line"
none = maybe(-1)
puts none
puts nil

puts "-- puts of untyped values"
puts ident(nil)
puts ident([1, ["", "x"]])
puts ident(Temp.new(3))
puts ident(:sym), ident(1.0), ident(false)

puts "-- a top-level Class.new block's constants are Object's; its hidden class is not"
Class.new do
  ANON_BLOCK_CONST = 1
end
p(Object.constants.include?(:ANON_BLOCK_CONST), Object.constants.none? { |k| k.to_s.start_with?("RbAnon") })

# A module method's super into another module's that answers another type (ruby/spec's AliasWithSuper, decision 416):
# reachable, so its bridge must build; not called, as rugaroo types super by the override's result.
module DynOutSupA
  def dyn_out_sup = :a
end

module DynOutSupB
  def dyn_out_sup = [:b, super]
end

class DynOutSupBase
  extend DynOutSupA
  extend DynOutSupB
end
dyn_out_sup_any = [DynOutSupBase, 1, "x", :y][0] #: untyped
p dyn_out_sup_any.dyn_out_sup if ARGV.size > 99

# `def initialize_copy` without the source object: dup passes it, MRI's ArgumentError (ruby/spec's def_spec, decision 416)
class DynOutCopy
  def initialize_copy
    @copied = true
  end
end
begin
  DynOutCopy.new.dup
rescue ArgumentError => e
  p e.message
end
