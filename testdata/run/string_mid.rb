# rbs_inline: enabled

# default inspect stays distinct from a user to_s
class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: () -> String
  def to_s = "Pt(#{x})"
end

puts (Pt.new(3).inspect == Pt.new(3).to_s).inspect
puts Pt.new(3).inspect.include?("@x=3").inspect

# a module's #{self} and bare to_s/inspect dispatch to the includer
module Tagged
  #: () -> String
  def tag = "[#{self}]"

  #: () -> String
  def loud = to_s.upcase

  #: () -> String
  def shown = "<#{inspect}>"
end

class Item
  include Tagged

  #: () -> String
  def to_s = "item"

  #: () -> String
  def inspect = "#<Item>"
end

puts Item.new.tag, Item.new.loud, Item.new.shown
