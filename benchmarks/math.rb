# rbs_inline: enabled

total = 0.0 #: Float
200_000.times { |i| total += Math.sin(i.to_f) * Math.cos(i.to_f) }
puts total.round(4)
