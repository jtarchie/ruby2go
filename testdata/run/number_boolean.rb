# rbs_inline: enabled

#: (bool) -> String
def yes_no(b) = b ? "yes" : "no"

#: (bool) -> String
def describe(b)
  case b
  when true then "on"
  when false then "off"
  else "?"
  end
end

#: (String, bool) -> bool
def noisy(tag, b)
  puts "eval #{tag}"
  b
end

#: (untyped) -> untyped
def ident(v) = v

t = true #: bool
f = false #: bool

puts "-- inspect and to_s"
puts t.inspect, f.inspect, t.to_s.inspect, f.to_s.inspect
puts t, f
puts "#{t}|#{f}"
print t, f, "\n"
puts [t, f, true, false].inspect

puts "-- ! and not"
puts (!t).inspect, (!f).inspect, (not t).inspect, (!!t).inspect, (!!f).inspect

puts "-- & | ^ evaluate both sides"
puts (t & f).inspect, (t & t).inspect, (f & f).inspect, (f & t).inspect
puts (t | f).inspect, (f | f).inspect, (t | t).inspect, (f | t).inspect
puts (t ^ f).inspect, (t ^ t).inspect, (f ^ f).inspect, (f ^ t).inspect

puts "-- == and != take any object"
puts (t == true).inspect, (t == false).inspect, (f == false).inspect, (f == true).inspect
puts (t == 1).inspect, (f == 0).inspect, (f == nil).inspect, (t == "true").inspect, (t == :true).inspect
puts (t != f).inspect, (t != true).inspect, (f != nil).inspect

puts "-- && || and conditions"
puts (t && f).inspect, (t || f).inspect, (f || t).inspect, (f && t).inspect, (f || f).inspect, (t && t).inspect
puts yes_no(t), yes_no(f), yes_no(1 < 2), yes_no(2.5 < 1.0), yes_no(1 == 1.0)
puts describe(t), describe(f), describe(1 > 2)
puts (1 < 2 && 2 < 3).inspect, (1 > 2 || 3 > 2).inspect, (!(1 < 2)).inspect
flag = false
3.times { |i| flag = !flag if i.even? }
puts flag.inspect
count = 0
count += 1 while count < 4 && !(count == 3)
puts count.inspect

puts "-- Kernel and BasicObject methods on bool"
puts t.nil?.inspect, t.frozen?.inspect, t.equal?(true).inspect, f.equal?(true).inspect
puts t.then { |b| !b }.inspect

puts "-- & | ^ always evaluate the right side; && || short-circuit"
puts (t | noisy("or", false)).inspect, (f & noisy("and", true)).inspect, (t ^ noisy("xor", true)).inspect
puts (t || noisy("never", false)).inspect, (f && noisy("never", true)).inspect, (f || noisy("rhs", true)).inspect

puts "-- op-assign"
b = true
b &= false
puts b.inspect
b |= true
puts b.inspect
b ^= true
puts b.inspect
b ||= true
puts b.inspect
c = nil #: bool?
c ||= false
puts c.inspect

puts "-- through untyped"
u = ident(true)
w = ident(false)
puts (!u).inspect, (!w).inspect, (u & w).inspect, (u | w).inspect, (u ^ w).inspect, u.to_s.inspect, w.inspect
puts (u == true).inspect, (w == false).inspect, (u != w).inspect, (w == nil).inspect, (u == 1).inspect
puts(u ? "yes" : "no")
puts(w ? "yes" : "no")
puts yes_no(ident(true)), yes_no(ident(nil)), yes_no(ident(0))
# An untyped argument to a bool parameter is tested for truthiness, as Ruby's & | ^ do.
puts (t & ident(1)).inspect, (t & ident(nil)).inspect, (f | ident("s")).inspect, (f ^ ident(0)).inspect, (t ^ ident(false)).inspect

puts "-- in collections"
flags = [true, false, true] #: Array[bool]
puts flags.select { |x| x }.size.inspect, flags.include?(false).inspect, flags.tally.inspect, flags.map(&:!).inspect
puts flags.all? { |x| x }.inspect, flags.any? { |x| !x }.inspect, flags.reject { |x| x }.inspect
bh = { "on" => true, "off" => false } #: Hash[String, bool]
puts bh["on"].inspect, bh["off"].inspect, bh["x"].inspect, bh.inspect
bk = { true => 1, false => 0 } #: Hash[bool, Integer]
puts bk[true].inspect, bk[1 > 2].inspect, bk.inspect
