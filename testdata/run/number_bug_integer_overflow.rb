# skip: wontfix: Integer is a 64-bit Go int (docs/design.md decision 35); where MRI promotes to a Bignum, rb2go raises RangeError
# rbs_inline: enabled

m = 9_223_372_036_854_775_807 #: Integer
puts (m + 1).inspect, (m * 2).inspect, (2 ** 63).inspect, (2 ** 64).inspect, (-m - 2).inspect
puts (-m - 1).abs.inspect, (-(-m - 1)).inspect, ((-m - 1) / -1).inspect, 1e19.to_i.inspect, 1e20.floor.inspect
f = 1 #: Integer
1.upto(25) { |i| f *= i }
puts f.inspect
puts "ffffffffffffffffffff".hex.inspect, "7fffffffffffffff".hex.inspect, "1777777777777777777777".oct.inspect, "zzzzzzzzzzzzzz".to_i(36).inspect
