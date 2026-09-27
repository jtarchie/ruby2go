# rbs_inline: enabled

# 0d decimal-prefix literals
puts 0d17.inspect, 0D0.inspect

# Integer#chr past 127 is one byte; out of range raises RangeError
puts 200.chr.bytesize.inspect, 255.chr.bytesize.inspect, 128.chr.bytesize.inspect, 127.chr.bytesize.inspect
puts 200.chr.ord.inspect, 255.chr.ord.inspect
begin
  puts 256.chr.bytesize.inspect
rescue RangeError => chr_err
  puts chr_err.message
end
begin
  puts -1.chr.bytesize.inspect
rescue RangeError => chr_err
  puts chr_err.message
end

# clamp with min > max raises ArgumentError
begin
  puts 5.clamp(10, 1).inspect
rescue ArgumentError => clamp_err
  puts clamp_err.message
end
begin
  puts 2.5.clamp(3.0, 1.0).inspect
rescue ArgumentError => clamp_err
  puts clamp_err.message
end

# Integer == falls back to the other side's == for a user object
class One
  #: (untyped) -> bool
  def ==(o) = o == 1
end

one = One.new
puts (1 == one).inspect, (1.0 == one).inspect, (2 == one).inspect, (one == 1).inspect

# <=> with NaN is nil
nan_zero = 0.0 #: Float
nan = nan_zero / nan_zero #: Float
puts (nan <=> 1.0).inspect, (1.0 <=> nan).inspect, (nan <=> nan).inspect

# Float#** rounds like C pow
third = 1.0 / 3.0 #: Float
puts (8.0 ** third).inspect, (27.0 ** third).inspect, (1000.0 ** third).inspect
puts (0.1 ** 4.0).inspect, (1.01 ** 3.0).inspect, (9.9 ** 4.0).inspect, (2.0 ** 2.5).inspect, (10.0 ** 2.5).inspect
pow_x = 1.1 #: Float
puts (pow_x ** 10).inspect
neg = -1.1 #: Float
puts (neg ** 3).inspect, ((neg - 1.4) ** -3.0).inspect, (0.5 ** -3.3).inspect, (1e10 ** 30.5).inspect, (1.0000001 ** 1e7).inspect, (7.5 ** -7.5).inspect

# Float#round rounds half away from zero
puts 2.5.round.inspect, 0.5.round.inspect, -2.5.round.inspect, -0.5.round.inspect, 1.5.round.inspect, 3.5.round.inspect

# Float#to_s switches to exponent form at 1e16, or integral values from 1e15
puts 1e15.inspect, 1.5e15.inspect, -1.2e15.inspect, 1234567890123456.0.to_s, 9999999999999998.0.inspect
puts 9_007_199_254_740_993.to_f.inspect
puts "#{2e15}"
puts 1234567890123456.7.inspect, 999999999999999.9.inspect, 100000000000000.0.inspect

# Integer/Float mixed operators on typed locals, and an Integer accumulator fed Floats
build_a = 7 #: Integer
build_f = 1.5 #: Float
puts (1 < 1.5).inspect, (2 ** 0.5).inspect, (build_a * build_f).inspect, (build_f + build_a).inspect, (build_a <=> build_f).inspect
puts 1.between?(0.5, 2.5).inspect, build_a.clamp(0, 2.5).inspect
total = 0
[1.5, 2.25].each { |add| total += add }
puts total.inspect

# Integer/Float mixed operators on untyped values
mix_v = 5 #: untyped
mix_w = 1.5 #: untyped
puts (mix_v + 1.5).inspect, (mix_v * mix_w).inspect, (mix_w + mix_v).inspect, (mix_v < mix_w).inspect

# Integer op Float literal, and Float#clamp with Integer bounds
mixed_a = 7 #: Integer
puts (mixed_a / 2.0).inspect, (1 + 2.0).inspect, (mixed_a * 1.0).inspect, (mixed_a - 0.0).inspect
mixed_x = 2.5 #: Float
puts mixed_x.clamp(1, 2).inspect, 0.5.clamp(1, 2).inspect

# && on numbers yields the right side
and_a = 1 #: Integer
and_v = and_a && 2
puts and_v.inspect
and_b = 2.5 #: Float
and_w = and_b && 1.0
puts and_w.inspect

# 0 and 0.0 are truthy
truth_x = 0 #: Integer
truth_f = 0.0 #: Float
puts "0 is truthy" if truth_x
puts(truth_f ? "0.0 is truthy" : "0.0 is falsy")
truth_n = 3
while truth_n
  truth_n -= 1
  break if truth_n.zero?
end
puts truth_n.inspect

# a parenthesized literal as a receiver
puts ((-2) ** 3).inspect
puts (3).inspect, (2.5).inspect, (-0).inspect
puts (7).even?.inspect, (1.5).floor.inspect
paren_a = 1 #: Integer
puts (paren_a && 2).inspect, paren_a.inspect

# <=> on untyped numbers

#: (untyped) -> untyped
def ident_cmp(v) = v

cmp_a = ident_cmp(3)
cmp_b = ident_cmp(4)
puts (cmp_a <=> cmp_b).inspect, (ident_cmp(2.5) <=> ident_cmp(2.5)).inspect
puts (cmp_a <=> 1).inspect, (ident_cmp(1.5) <=> 2.0).inspect

# #hash on untyped values matches the typed hash

#: (untyped) -> untyped
def ident_hash(v) = v

hash_v = ident_hash(5)
hash_w = ident_hash("s")
puts (hash_v.hash == 5.hash).inspect, (hash_v.hash == 6.hash).inspect, (hash_w.hash == "s".hash).inspect

# a local or a branch holding both Integer and Float is untyped, as MRI has it
join_x = 1
join_x = 2.5
puts join_x
join_c = true #: bool
join_d = false #: bool
puts (join_c ? 1 : 2.5).inspect, (join_d ? 1 : 2.5).inspect
join_y = join_d ? 2.5 : 1
puts (join_y + 1).inspect
