# rbs_inline: enabled

x = 2.5 #: Float
y = -1.25 #: Float
z = 0.0 #: Float
inf = 1.0 / z #: Float
nan = z / z #: Float

puts "-- arithmetic"
puts (x + y).inspect, (x - y).inspect, (x * y).inspect, (x / y).inspect, (y / x).inspect
puts (x ** 2.0).inspect, (2.0 ** 0.5).inspect, (4.0 ** -1.0).inspect, (x ** 0.0).inspect, (9.0 ** 0.5).inspect
puts (0.1 + 0.2).inspect, (1.1 + 2.2).inspect, (0.1 * 3.0).inspect, (1.0 / 3.0).inspect, (2.0 / 3.0).inspect, (10.0 / 4.0).inspect
puts (-x).inspect, (-y).inspect, (-z).inspect, (z * -1.0).inspect
puts x.abs.inspect, y.abs.inspect, (-z).abs.inspect, (-inf).abs.inspect

puts "-- Integer literals where a Float is expected"
puts (x + 1).inspect, (x * 2).inspect, (x / 2).inspect, (x ** 2).inspect, (x - 3).inspect

puts "-- division by zero is IEEE, not an exception"
puts inf.inspect, (-1.0 / z).inspect, nan.inspect, (1.0 / (-z)).inspect, (inf - inf).inspect, (inf * z).inspect
puts (x / 0).inspect, (y / 0).inspect, (inf + 1.0).inspect, (1.0 / inf).inspect

puts "-- <=> and relational operators"
puts (x <=> 3.0).inspect, (x <=> 2.5).inspect, (x <=> y).inspect, (z <=> -z).inspect, (inf <=> x).inspect
puts (x < 3.0).inspect, (x < 2.5).inspect, (x <= 2.5).inspect, (x > 2.5).inspect, (x > y).inspect, (x >= 3.0).inspect, (x >= 2.5).inspect
puts (nan < 1.0).inspect, (nan > 1.0).inspect, (nan <= nan).inspect, (-inf < y).inspect

puts "-- == and != across Float and Integer"
puts (x == 2.5).inspect, (2.0 == 2).inspect, (x == 2).inspect, (-z == z).inspect
puts (x == "2.5").inspect, (x == nil).inspect, (x == true).inspect, (nan == nan).inspect, (inf == inf).inspect
puts (x != 2.5).inspect, (x != 3.0).inspect, (nan != nan).inspect
w = 7 #: Integer
puts (7.0 == w).inspect, (w.to_f == 7.0).inspect, (w.to_f / 2.0).inspect

puts "-- to_i truncates, floor and ceil return Integer"
puts x.to_i.inspect, y.to_i.inspect, 1.99.to_i.inspect, -1.99.to_i.inspect, -0.5.to_i.inspect, 1e10.to_i.inspect
puts x.floor.inspect, y.floor.inspect, 3.0.floor.inspect, -3.0.floor.inspect, -0.5.floor.inspect
puts x.ceil.inspect, y.ceil.inspect, 3.0.ceil.inspect, -3.0.ceil.inspect, -0.5.ceil.inspect, 0.1.ceil.inspect

puts "-- round (halves are in number_bug_float_round_half)"
puts 1.4.round.inspect, 1.6.round.inspect, -1.4.round.inspect, -1.6.round.inspect, 3.0.round.inspect, 2.49999.round.inspect, 0.2.round.inspect
# Guards against floor(x + 0.5): the largest double below 0.5 rounds to 0.
puts 0.49999999999999994.round.inspect, 2.5000000000000004.round.inspect, -0.4.round.inspect, 4503599627370497.0.round.inspect

puts "-- predicates and to_f"
puts x.to_f.inspect, z.zero?.inspect, (-z).zero?.inspect, x.zero?.inspect, inf.zero?.inspect
puts x.nan?.inspect, nan.nan?.inspect, inf.nan?.inspect

puts "-- to_s and inspect"
puts 6.0.inspect, 6.0.to_s, 100.0.inspect, 1.5.to_s, -2.75.inspect, -0.0.inspect, 0.0.to_s
puts 1e2.inspect, 12.0e0.inspect, 1_000.5.inspect, 1e3.inspect, 2E-2.inspect, 3.14159.inspect
puts 123456789.123456789.inspect, 123456789012345.6.inspect, 999999999999999.0.inspect, 100000000000000.5.inspect
# Non-integral values below 1e16 stay fixed (a guard for the 1e15 fix in number_bug_float_to_s_1e15).
puts 1234567890123456.7.inspect, -999999999999999.9.inspect
puts 1e16.inspect, -1e16.inspect, 1.0e20.inspect, 1.5e300.inspect, 1.7976931348623157e308.inspect
puts 0.001.inspect, 0.0001.inspect, 0.00011.inspect, 0.00001.inspect, 0.000123.inspect, 1.23e-5.inspect, -1.23e-5.inspect, 5e-324.inspect
puts inf.to_s, (-inf).to_s, nan.to_s
puts x, y, z
puts "#{x}|#{y}|#{-0.0}|#{1e20}|#{inf}"
print x, y, "\n"
puts [x, y, z, 2.5e20, -0.0, inf, nan].inspect

puts "-- op-assign"
f = 1.5
f += 1.0
f -= 0.25
f *= 4.0
f /= 3.0
f **= 2.0
puts f.inspect

puts "-- Kernel methods on Float"
puts x.then { |v| v * 2.0 }.inspect, x.frozen?.inspect, x.nil?.inspect, (!x).inspect
