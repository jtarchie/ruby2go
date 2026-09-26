# skip: Struct/Data == uses is_a? on the declaring class, so a subclass instance equals a parent instance with the same members; MRI requires the same class

# rbs_inline: enabled

Point = Struct.new(:x, :y) #: [Integer, Integer]

class Point3 < Point
end

Val = Data.define(:v) #: [Integer]

class SubVal < Val
end

puts (Point.new(1, 2) == Point3.new(1, 2)).inspect
puts (Point3.new(1, 2) == Point.new(1, 2)).inspect
puts (Point3.new(1, 2) == Point3.new(1, 2)).inspect
puts (Val.new(1) == SubVal.new(1)).inspect
puts (SubVal.new(1) == Val.new(1)).inspect
puts SubVal.new(1).inspect, SubVal.new(2).with(v: 3).inspect
