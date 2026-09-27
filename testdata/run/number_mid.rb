# rbs_inline: enabled

# NaN in between?/clamp/sort/max raises like MRI instead of comparing
nz = 0.0 #: Float
nan = nz / nz #: Float
begin
  puts nan.between?(0.0, 1.0).inspect
rescue ArgumentError => err
  puts "between? #{err.class}"
end
begin
  puts nan.clamp(0.0, 1.0).inspect
rescue ArgumentError => err
  puts "clamp #{err.class}"
end
begin
  puts [1.0, nan].sort.inspect
rescue ArgumentError => err
  puts "sort #{err.class}"
end
begin
  puts [nan, 1.0].max.inspect
rescue ArgumentError => err
  puts "max #{err.class}"
end

# != follows ==, across Integer/Float and a user-defined ==
class Money
  attr_reader :cents #: Integer

  #: (Integer) -> void
  def initialize(cents)
    @cents = cents
  end

  #: (untyped) -> bool
  def ==(other) = other.is_a?(Money) && other.cents == cents
end

x = 1 #: Integer
y = 1.0 #: Float
puts (x != 1.0).inspect, (y != 1).inspect, (x != y).inspect, (y != x).inspect
puts (Money.new(5) == Money.new(5)).inspect, (Money.new(5) != Money.new(5)).inspect

# a bool? holding false is as falsy as nil

#: (bool) -> bool?
def maybe(b) = b ? false : nil

ob_c = maybe(true)
puts (ob_c || "fallback").inspect
puts(ob_c ? "truthy" : "falsy")
puts "not printed" if ob_c
ob_d = maybe(true)
ob_d ||= true
puts ob_d.inspect
ob_e = maybe(true)
puts (ob_e && "rhs").inspect
ob_h = { "off" => false } #: Hash[String, bool]
puts (ob_h["off"] || true).inspect
puts(ob_h["off"] ? "on" : "off")
puts(ob_h["missing"] ? "on" : "missing")

# += on a user class calls its own +
class Num
  attr_reader :v #: Integer

  #: (Integer) -> void
  def initialize(v)
    @v = v
  end

  #: (Num) -> Num
  def +(o) = Num.new(v + o.v)
end

num_c = Num.new(1) #: Num
num_c += Num.new(2)
num_c += Num.new(3)
puts num_c.v.inspect
num_d = Num.new(10)
num_d = num_d + Num.new(5)
puts num_d.v.inspect

# results at Integer's 64-bit edges must not trip the overflow checks (decision 35)
m = 0x7fff_ffff_ffff_ffff #: Integer
n = -9_223_372_036_854_775_808 #: Integer
t = -2 #: Integer
o = -1 #: Integer
puts m + 0, n + 0, m - 0, (n + 1) - 1, m + n, n - -m, 0 - m
puts 3_037_000_499 * 3_037_000_499, -3_037_000_499 * 3_037_000_499
puts(-4_611_686_018_427_387_904 * 2, 4_611_686_018_427_387_903 * 2)
puts n * 1, m * -1, -1 * m, 2_147_483_648 * 2_147_483_647, -2_147_483_648 * -2_147_483_648
puts 1_099_511_627_776 * 4_194_304, -1_099_511_627_776 * 8_388_608
puts 2 ** 62, t ** 63, 3 ** 39, 10 ** 18, (t - 1) ** 3, 7 ** 0
puts 1 ** 1_000_000_000_000, o ** 1_000_000_000_001, o ** -4, o ** -3, 1 ** -5
puts (-m).abs, m.pred.succ, n / 1, n / 2, n % -1, -m / -1
puts 9.2e18.to_i, -9.2e18.floor, "-9223372036854775808".to_i, "9223372036854775807".to_i
