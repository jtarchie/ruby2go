# skip: a trailing `#: T` on `@x ||= []` / `@x ||= {}` is ignored, so the ivar is Array[untyped] / Hash[untyped, untyped] and go build fails "cannot use ... *Array[any] as *Array[String]"; a class-level ivar has no other way to be annotated

# rbs_inline: enabled

class Box
  #: () -> Array[String]
  def items
    @items ||= [] #: Array[String]
  end
end

class Reg
  #: () -> Hash[String, Integer]
  def self.table
    @table ||= {} #: Hash[String, Integer]
  end
end

b = Box.new
b.items << "a"
Reg.table["x"] = 1
puts b.items.inspect, Reg.table.inspect
