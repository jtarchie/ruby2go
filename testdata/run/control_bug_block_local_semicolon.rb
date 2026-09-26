# skip: block-local variables declared after `;` in the block parameters (`|v; x|`) are ignored, so assigning them overwrites the outer local of the same name

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
