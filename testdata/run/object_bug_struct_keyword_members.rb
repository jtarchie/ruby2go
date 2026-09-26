# skip: a Struct/Data member named like a Ruby keyword (:begin, :end, :in) is an internal error: the generated initialize uses the member names as parameter names and does not parse

# rbs_inline: enabled

Span = Struct.new(:begin, :end) #: [Integer, Integer]
Link = Data.define(:in, :out) #: [String, String]

s = Span.new(1, 5)
puts s.begin.inspect, s.end.inspect, s.inspect, s.to_a.inspect, s.to_h.inspect
s.end = 7
puts (s == Span.new(1, 7)).inspect
l = Link.new(in: "a", out: "b")
puts l.in, l.inspect, l.with(out: "c").inspect
