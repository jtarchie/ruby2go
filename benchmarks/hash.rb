# rbs_inline: enabled

counts = {} #: Hash[Integer, Integer]
500_000.times do |i|
  key = i % 1000
  counts[key] = (counts[key] || 0) + 1
end
puts counts.size
puts counts.values.sum
