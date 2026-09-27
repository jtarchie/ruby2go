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
