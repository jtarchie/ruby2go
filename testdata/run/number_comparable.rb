# rbs_inline: enabled

# A method added to Comparable reaches Integer and Float, and reopened classes call mixed-in methods on self.
module Comparable
  #: (self) -> self
  def at_least(o) = self < o ? o : self
end

class Float
  #: () -> bool
  def unit? = between?(0.0, 1.0)
end

class Integer
  #: () -> Integer
  def digit = clamp(0, 9)
end

#: (untyped) -> untyped
def ident(v) = v

a = 7 #: Integer
b = -7 #: Integer
x = 2.5 #: Float

puts "-- Comparable#between? is inclusive"
puts a.between?(1, 7).inspect, a.between?(7, 9).inspect, a.between?(8, 9).inspect, a.between?(1, 6).inspect
puts b.between?(-7, -7).inspect, b.between?(-8, 0).inspect, 0.between?(b, a).inspect, a.between?(9, 1).inspect
puts x.between?(2.0, 3.0).inspect, x.between?(2.5, 2.5).inspect, x.between?(3.0, 4.0).inspect, x.between?(-1.0, 2.4).inspect

puts "-- Comparable#clamp"
puts a.clamp(1, 5).inspect, a.clamp(8, 10).inspect, a.clamp(1, 10).inspect, a.clamp(7, 7).inspect, b.clamp(-3, 3).inspect
puts x.clamp(0.0, 1.0).inspect, x.clamp(3.0, 4.0).inspect, x.clamp(1.0, 4.0).inspect, (-x).clamp(-1.0, 1.0).inspect

puts "-- <=> drives sorting and min/max"
ints = [3, -1, 0, 10, -20, 3] #: Array[Integer]
floats = [2.5, -0.5, 1.0, -3.25, 1e20, 0.0] #: Array[Float]
puts ints.sort.inspect, floats.sort.inspect
puts ints.max.inspect, ints.min.inspect, floats.max.inspect, floats.min.inspect
puts ints.sort.reverse.inspect, floats.sort.reverse.inspect
puts ints.sort_by { |i| -i }.inspect, ints.min_by { |i| i.abs }.inspect, floats.max_by { |f| f.abs }.inspect
puts [2.0, 1.0].sort.inspect, ints.map { |i| i <=> 3 }.inspect, floats.map { |f| f <=> 1.0 }.inspect

puts "-- mixed-in methods chained, in blocks, and on reopened classes"
puts a.clamp(1, 5).succ.inspect, x.clamp(0.0, 1.0).floor.inspect, a.clamp(1, 9).between?(6, 8).inspect
puts ints.select { |i| i.between?(0, 5) }.inspect, floats.map { |f| f.clamp(-1.0, 1.0) }.inspect
puts 3.at_least(5).inspect, 7.at_least(5).inspect, 1.5.at_least(0.5).inspect, 1.5.at_least(2.5).inspect
puts 0.5.unit?.inspect, 1.5.unit?.inspect, 12.digit.inspect, -3.digit.inspect, 4.digit.inspect
lo = 2 #: Integer
hi = 4 #: Integer
puts 3.between?(lo, hi).inspect, 9.clamp(lo, hi).inspect, (lo + hi).clamp(lo, hi * 2).inspect

puts "-- through untyped"
v = ident(5)
w = ident(2.5)
puts v.between?(1, 10).inspect, v.clamp(1, 3).inspect, w.between?(3.0, 4.0).inspect, w.clamp(0.0, 1.0).inspect
