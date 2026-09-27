# rbs_inline: enabled

Span = Struct.new(:begin, :end) #: [Integer, Integer]
Link = Data.define(:in, :out) #: [String, String]

s = Span.new(1, 5)
puts s.begin.inspect, s.end.inspect, s.inspect, s.to_a.inspect, s.to_h.inspect
s.end = 7
puts (s == Span.new(1, 7)).inspect
l = Link.new(in: "a", out: "b")
puts l.in, l.inspect, l.with(out: "c").inspect
