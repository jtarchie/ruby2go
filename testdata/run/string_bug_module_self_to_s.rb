# skip: to_s/inspect on self inside a module method ("#{self}", to_s.upcase) emit self.ToS() on the generic Self, whose derived constraint lacks ToS (go build: type Self has no field or method ToS)

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
