# rbs_inline: enabled

# Integer?/Float?/bool? values (from hashes and methods), narrowing, and numbers held in collections.

#: (Array[Integer], Integer) -> Integer?
def find_over(xs, n)
  xs.each { |x| return x if x > n }
  nil
end

#: (Integer?) -> Integer
def double_or_zero(x)
  return 0 unless x
  x * 2
end

#: (Float?) -> Float
def half(x)
  return -1.0 if x.nil?
  x / 2.0
end

#: (Integer?) -> String
def describe(x)
  if x && x > 10
    "big #{x}"
  elsif x
    "small #{x.succ}"
  else
    "none"
  end
end

ints = [5, 3, 8, 1] #: Array[Integer]
floats = [1.5, -2.25, 4.0] #: Array[Float]

puts "-- Integer? from a method"
r = find_over(ints, 4)
q = find_over(ints, 100)
puts r.inspect, q.inspect, (r || 0).inspect, (q || -1).inspect, q.nil?.inspect, r.nil?.inspect
puts r&.succ.inspect, q&.succ.inspect
puts double_or_zero(r).inspect, double_or_zero(q).inspect, half(3.0).inspect, half(nil).inspect
puts describe(42), describe(3), describe(nil)

puts "-- Hash lookups are T?"
h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
fh = { "pi" => 3.14 } #: Hash[String, Float]
bh = { "on" => true } #: Hash[String, bool]
x = h["zz"]
y = fh["zz"]
z = bh["zz"]
puts h["a"].inspect, x.inspect, (x || 0).inspect, fh["pi"].inspect, y.inspect, (y || 2.72).inspect, z.inspect
puts x, y, z
puts "[#{x}][#{y}][#{z}][#{h["a"]}][#{fh["pi"]}][#{bh["on"]}]"
print x, y, z, "\n"
puts x.to_s.inspect, y.to_s.inspect, z.to_s.inspect
puts (!x).inspect, (!!h["a"]).inspect, (!h["a"]).inspect
puts (x == nil).inspect, (h["a"] == 1).inspect, (h["a"] == 1.0).inspect, (x == 0).inspect, (fh["pi"] == 3.14).inspect, (bh["on"] == true).inspect
w = h["b"]
puts (w + 10).inspect if w
puts w&.even?.inspect
c = h["q"]
c ||= 7
puts (c + 1).inspect
d = (h["a"] || 0) + 100
puts d.inspect
e = nil #: Float?
e = 2.5 if h.size > 0
puts (e ? e * 2.0 : 0.0).inspect
counts = {} #: Hash[String, Integer]
%w[a b a c a].each { |k| counts[k] = (counts[k] || 0) + 1 }
puts counts.inspect

puts "-- numbers as hash keys"
mk = { 1 => "int", 1.0 => "float", true => "bool" } #: Hash[untyped, String]
puts mk.inspect, mk.size.inspect, mk[1].inspect, mk[1.0].inspect, mk[true].inspect, mk[2].inspect
fk = { 0.5 => "half", -1.25 => "neg" } #: Hash[Float, String]
puts fk[0.5].inspect, fk[-1.25].inspect, fk[2.0].inspect, fk.keys.inspect
zk = { 0.0 => "zero" } #: Hash[Float, String]
puts zk[-0.0].inspect
ik = { 1 => 10, 2 => 20 } #: Hash[Integer, Integer]
puts ik.map { |k, v| k * v }.inspect, ik.keys.map(&:to_f).inspect, ik.values.max.inspect

puts "-- Enumerable over numbers"
puts ints.reduce(0) { |s, v| s + v }.inspect, floats.reduce(0.0) { |s, v| s + v }.inspect, ints.inject(1) { |s, v| s * v }.inspect
puts ints.map { |v| v.to_f / 2.0 }.inspect, floats.map { |v| v.round }.inspect, floats.map(&:floor).inspect, floats.map(&:ceil).inspect
puts ints.select(&:even?).inspect, ints.reject(&:odd?).inspect, ints.map(&:succ).inspect, ints.map(&:-@).inspect
puts ints.include?(8).inspect, ints.include?(9).inspect, floats.include?(4.0).inspect, floats.include?(4.5).inspect
puts ints.tally.inspect, [1, 1, 2].tally.inspect, [1.5, 1.5, -0.5].uniq.inspect, [1, 2, 1].uniq.inspect
puts ints.any?(&:zero?).inspect, ints.all?(&:positive?).inspect, floats.none?(&:nan?).inspect
puts ints.group_by(&:odd?).inspect, ints.min_by(&:abs).inspect, floats.sort_by { |f| -f }.inspect
puts [72, 105].map(&:chr).join, ints.map(&:to_s).join("+")
puts ints.sort.first(2).inspect, floats.min.floor.inspect, ints.max.succ.inspect
