# rbs_inline: enabled

xs = [1, 2, 3] #: Array[Integer]
ys = xs.map do |x|
  next if x == 2
  x * 10
end
puts ys.inspect
picked = xs.select do |x|
  next if x.odd?
  true
end
puts picked.inspect
