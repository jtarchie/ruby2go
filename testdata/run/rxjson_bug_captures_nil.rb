# skip: a nil element of an Array[T?] (MatchData#captures, a destructured capture, map { =~ }) panics on a nil *T in inspect and join instead of printing nil / ""

# rbs_inline: enabled

m = /(a)(b)?/.match("a")
if m
  c = m.captures
  puts c.size
  puts c[1].inspect
  k, v = m.captures
  puts k.inspect, v.inspect
  puts c.inspect
  puts c.join(","), m.captures.join("-").size
end
puts %w[cat dog].map { |w| w =~ /a/ }.inspect
