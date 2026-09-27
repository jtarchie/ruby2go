# rbs_inline: enabled
a = []
a << 3
a << 1
a << 2
puts a.sort.inspect, a.min.inspect, a.max.inspect, a.sort_by { |x| x }.inspect
