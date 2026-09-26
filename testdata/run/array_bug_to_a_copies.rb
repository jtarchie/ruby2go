# skip: Array#to_a returns a copy (Enumerable#to_a); MRI returns self

# rbs_inline: enabled
a = [1, 2] #: Array[Integer]
b = a.to_a
b << 3
puts a.inspect, a.to_a.equal?(a)
