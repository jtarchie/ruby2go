# rbs_inline: enabled

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
