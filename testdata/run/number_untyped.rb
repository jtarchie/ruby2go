# rbs_inline: enabled

#: (untyped) -> String
def kind(v)
  case v
  when Integer then "Integer #{v.inspect}"
  when Float then "Float #{v.inspect}"
  when String then "String #{v.inspect}"
  when nil then "nil"
  else "other #{v.inspect}"
  end
end

#: (untyped) -> String
def truthy(v) = v ? "truthy" : "falsy"

#: (untyped) -> untyped
def ident(v) = v

puts "-- literals box to their Ruby class under untyped"
vals = [1, -2.5, true, false, nil, "s", 0, 0.0] #: Array[untyped]
vals.each { |v| puts kind(v) }
puts vals.inspect
puts kind(3), kind(-0.0), kind(1 == 1), kind(1 > 2), kind(7 / 2), kind(7.0 / 2.0)

puts "-- only nil and false are falsy; 0 and 0.0 are truthy"
vals.each { |v| puts truthy(v) }

puts "-- values round-trip through untyped"
i = ident(42) #: Integer
fl = ident(-1.5) #: Float
bo = ident(false) #: bool
puts (i + 1).inspect, (fl * 2.0).inspect, (!bo).inspect

puts "-- == across the boundary"
one = 1.0 #: untyped
puts (1 == one).inspect, (one == 1).inspect, (2 == one).inspect, (1.0 == ident(1)).inspect, (true == ident(true)).inspect, (false == ident(nil)).inspect

puts "-- is_a? and class"
puts 1.is_a?(Integer).inspect, 1.is_a?(Comparable).inspect, 1.is_a?(Float).inspect, 1.kind_of?(Object).inspect
puts 1.5.is_a?(Float).inspect, 1.5.is_a?(Comparable).inspect, 1.5.is_a?(Integer).inspect
puts ident(1).is_a?(Integer).inspect, ident(1.5).is_a?(Integer).inspect, ident(1.5).is_a?(Float).inspect
puts 1.class.inspect, 1.5.class.inspect, 1.class.name.inspect

puts "-- dynamic calls on untyped numbers (values from ident are real Go `any`s; <=> is in number_bug_untyped_cmp)"
v = ident(5)
w = ident(2.5)
puts (v + 1).inspect, (v - 8).inspect, (v * 3).inspect, (v / 2).inspect, (v % 3).inspect, (v ** 2).inspect, (-v).inspect
puts (w * 2.0).inspect, (w - 0.5).inspect, (w / 2.0).inspect, (-w).inspect, (w + w).inspect, (v * v).inspect
puts (v < 10).inspect, (v >= 5).inspect, (w > 3.0).inspect, (v < v).inspect, (w <= w).inspect
puts v.even?.inspect, v.odd?.inspect, v.zero?.inspect, v.positive?.inspect, v.negative?.inspect, v.succ.inspect, v.pred.inspect, v.chr.inspect
puts v.abs.inspect, w.floor.inspect, w.ceil.inspect, w.to_i.inspect, v.to_f.inspect, w.abs.inspect, w.nan?.inspect, w.zero?.inspect
puts v.to_s.inspect, w.to_s.inspect, v.inspect, w.inspect, "#{v}|#{w}|#{ident(-0.0)}|#{ident(1e20)}"
puts (v == 5).inspect, (w == 2.5).inspect, (v == w).inspect, (v == ident(5.0)).inspect, (ident(5.0) != v).inspect
total = 0
[ident(1), ident(2), ident(3)].each { |x| total += x }
puts total.inspect
h = { "n" => 1, "f" => 2.5, "b" => true } #: Hash[String, untyped]
puts h.inspect, (h["f"] * 2.0).inspect
puts [1e20, -0.0, 100.0, 1e-5, 7, -3, true, nil].inspect
