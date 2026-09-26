# rbs_inline: enabled

#: (String?) -> bool
def blank?(s) = s.nil? || s.empty?
puts blank?(nil), blank?(""), blank?("x")

#: (Integer?) -> bool
def big?(n) = !n.nil? && n > 10
puts big?(nil), big?(5), big?(50)


#: (Integer?, Integer) -> Integer
def pick(a, b)
  return a || b
end
puts pick(nil, 2), pick(1, 2)

flag = true
opt = nil #: Integer?
mixed = flag && opt
puts mixed.inspect
mixed2 = !flag || opt
puts mixed2.inspect

values = [opt || 1, (flag && 2) || 3] #: Array[Integer]
puts values.inspect

#: (Integer) -> String
def show(n) = "n=#{n}"
puts show(opt || 5)

u = nil #: untyped
w = 3 #: untyped
puts (u || "dflt").inspect, (w || "dflt").inspect, (u && "x").inspect, (w && "x").inspect

name = nil #: String?
greeting = name && "hi #{name}"
puts greeting.inspect
name = "zed"
greeting = name && "hi #{name}"
puts greeting.inspect

# && binding tighter than ||, and `and`/`or` equal precedence left to right
t = true
f = false
puts (t || f && f).inspect
r = (f or t and f)
puts r.inspect

# assignment inside && in a condition
store = { "k" => 5 } #: Hash[String, Integer]
if flag && (got = store["k"])
  puts "got #{got}"
end

# || on an optional with a side-effecting right side, only when nil
hits = 0
3.times do |i|
  cached = (i == 1 ? 7 : nil) #: Integer?
  val = cached || (hits += 1)
  puts val
end
puts hits

# not / ! on optional and untyped values
puts (not opt).inspect, (!u).inspect, (!w).inspect

# multiple assignment from Arrays, untyped values and optionals
kname, kval = "a=1".split("=")
puts kname.inspect, kval.inspect
k2, v2 = "novalue".split("=")
puts k2.inspect, v2.inspect
un = [1, "two"] #: Array[untyped]
u1, u2, u3 = un
puts u1.inspect, u2.inspect, u3.inspect
o1 = 1 #: Integer?
o2 = nil #: Integer?
o1, o2 = o2, o1
puts o1.inspect, o2.inspect
h = { "x" => 1, "y" => 2 } #: Hash[String, Integer]
h.each do |pair|
  kk, vv = pair
  puts "#{kk}:#{vv}"
end
h.to_a.each do |kk, vv|
  puts kk + vv.to_s
end
ix = 0
while ix < 3
  ix, dummy = ix + 1, ix
end
puts ix
# multiple assignment inside a closure block writes outer locals
lo = 0
hi = 0
pairs = [[1, 9], [2, 8]] #: Array[[Integer, Integer]]
pairs.each { |pair| lo, hi = pair[1], pair[0] }
puts lo.inspect, hi.inspect
ww = 0
res = [5].map { |x| ww, z = x, x * 2; z }
puts ww, res.inspect
