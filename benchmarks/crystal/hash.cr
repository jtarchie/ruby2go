# Crystal port of benchmarks/hash.rb, for the README comparison column.

counts = {} of Int32 => Int32
500_000.times do |i|
  key = i % 1000
  counts[key] = (counts[key]? || 0) + 1
end
puts counts.size
puts counts.values.sum
