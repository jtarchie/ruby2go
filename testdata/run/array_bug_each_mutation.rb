# rbs_inline: enabled
a = [1, 2] #: Array[Integer]
a.each do |x|
  a << x * 10 if x < 10
  puts x
end
puts a.inspect
d = [1, 2, 3, 4] #: Array[Integer]
seen = [] #: Array[Integer]
d.each do |x|
  seen << x
  d.delete(x)
end
puts seen.inspect, d.inspect
c = [1, 2] #: Array[Integer]
c.each do |x|
  c.clear
  puts x
end
