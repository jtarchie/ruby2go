# rbs_inline: enabled

# ys is printed without Array[Integer?]#inspect (see control_bug_next_in_value_block)
xs = [1, 2, 3] #: Array[Integer]
ys = xs.map do |x|
  next if x == 2
  x * 10
end
puts ys.map { |y| y ? y.to_s : "nil" }.join(",")
picked = xs.select do |x|
  next if x.odd?
  true
end
puts picked.inspect
puts xs.map { next }.inspect
ws = xs.map do |x|
  next if x == 1
  puts x
end
puts ws.inspect
