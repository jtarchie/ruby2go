# rbs_inline: enabled

x = 5
[1, 2].each { |v; x| x = v * 100 }
puts x
total = 0
sums = [3, 4].map do |v; total|
  total = v + 1
  total
end
puts sums.inspect, total
