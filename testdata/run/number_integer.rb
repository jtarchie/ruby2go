# rbs_inline: enabled

# Ends with an uncaught ZeroDivisionError: exit code 1 and the stdout before it are compared.

a = 7 #: Integer
b = -7 #: Integer
z = 0 #: Integer

puts "-- + - * and unary minus"
puts (a + 3).inspect, (a - 10).inspect, (a * -3).inspect, (z * b).inspect, (b + b).inspect
puts (-a).inspect, (-b).inspect, (-z).inspect, (- -a).inspect
puts (-2 ** 2).inspect # -(2 ** 2)
puts (-2.abs).inspect # (-2).abs: the minus belongs to the literal

puts "-- / floors toward negative infinity"
puts (a / 2).inspect, (b / 2).inspect, (a / -2).inspect, (b / -2).inspect
puts (z / 5).inspect, (-6 / 3).inspect, (6 / -3).inspect, (1 / 7).inspect, (-1 / 7).inspect

puts "-- % takes the divisor's sign"
puts (a % 3).inspect, (b % 3).inspect, (a % -3).inspect, (b % -3).inspect
puts (-6 % 3).inspect, (z % 7).inspect, (-1 % 7).inspect, (1 % -7).inspect

puts "-- ** with non-negative exponents"
puts (2 ** 10).inspect, (0 ** 0).inspect, (5 ** 0).inspect, (0 ** 3).inspect, (1 ** 100).inspect
puts (b ** 3).inspect, (b ** 2).inspect, (2 ** 62).inspect

puts "-- abs, succ, pred"
puts a.abs.inspect, b.abs.inspect, z.abs.inspect
puts a.succ.inspect, a.pred.inspect, b.succ.inspect, z.pred.inspect, -1.succ.inspect

puts "-- <=> and relational operators"
puts (a <=> 8).inspect, (a <=> 7).inspect, (a <=> 6).inspect, (b <=> a).inspect, (a <=> b).inspect
puts (a < 8).inspect, (a < 7).inspect, (a <= 7).inspect, (a <= 6).inspect
puts (a > 7).inspect, (a > 6).inspect, (a >= 8).inspect, (a >= 7).inspect, (b < z).inspect

puts "-- == and != (Integer#== takes any object)"
puts (a == 7).inspect, (a == 8).inspect, (a == 7.0).inspect, (a == 7.5).inspect
puts (a == "7").inspect, (a == nil).inspect, (a == true).inspect, (z == 0.0).inspect, (z == -0.0).inspect
puts (a != 7).inspect, (a != 8).inspect, (a != "7").inspect

puts "-- even? odd? zero? positive? negative?"
puts 4.even?.inspect, 4.odd?.inspect, b.even?.inspect, b.odd?.inspect, z.even?.inspect, -4.even?.inspect, -3.odd?.inspect
puts z.zero?.inspect, a.zero?.inspect, b.zero?.inspect
puts a.positive?.inspect, b.positive?.inspect, z.positive?.inspect
puts a.negative?.inspect, b.negative?.inspect, z.negative?.inspect

puts "-- to_i, to_f, to_s, inspect, hash"
puts a.to_i.inspect, b.to_i.inspect
puts a.to_f.inspect, b.to_f.inspect, z.to_f.inspect, 1_000_000.to_f.inspect
puts a.to_s.inspect, b.to_s.inspect, z.to_s.inspect, a.inspect.inspect, b.inspect.inspect
puts a, b, z
puts "#{a}|#{b}|#{z}"
print a, b, z, "\n"
puts (a.hash == 7.hash).inspect, (a.hash == b.hash).inspect

puts "-- chr in ASCII"
puts 65.chr.inspect, 97.chr.inspect, 48.chr.inspect, 126.chr.inspect, 32.chr.inspect, 10.chr.inspect, 0.chr.inspect

puts "-- literals"
puts 1_000_000.inspect, 0xff.inspect, 0XFF.inspect, 0b1010.inspect, 0o17.inspect, 017.inspect, -0x10.inspect, 0.inspect
puts 9_223_372_036_854_775_807.inspect, -9_223_372_036_854_775_807.inspect, (-9_223_372_036_854_775_807 - 1).inspect

puts "-- op-assign"
x = 10
x += 5
x -= 3
x *= 2
x /= 5
x %= 3
puts x.inspect
x = -3
x **= 3
puts x.inspect
x /= 2
puts x.inspect

puts "-- Kernel and BasicObject methods on Integer"
puts a.then { |v| v * 2 }.inspect, a.equal?(7).inspect, a.equal?(8).inspect
puts a.frozen?.inspect, a.nil?.inspect, (!a).inspect, (!z).inspect

puts "-- ZeroDivisionError"
begin
  puts (a / z).inspect
rescue ZeroDivisionError => e
  puts "/ #{e.message} #{e.class}"
end
begin
  puts (a % z).inspect
rescue ZeroDivisionError => e
  puts "% #{e.message}"
end
begin
  puts (z / z).inspect
rescue StandardError => e
  puts "0/0 #{e.message}"
end
puts "uncaught next"
puts (b / z).inspect
puts "not reached"
