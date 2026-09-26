# skip: Integer op non-integral Float (or a Float-typed value) fails go build (constant 1.5 truncated / cannot use Float as Integer); MRI promotes to Float
# rbs_inline: enabled

a = 7 #: Integer
f = 1.5 #: Float
puts (1 < 1.5).inspect, (2 ** 0.5).inspect, (a * f).inspect, (f + a).inspect, (a <=> f).inspect
puts 1.between?(0.5, 2.5).inspect, a.clamp(0, 2.5).inspect
# An accumulator started at 0 and fed Floats.
total = 0
[1.5, 2.25].each { |x| total += x }
puts total.inspect
