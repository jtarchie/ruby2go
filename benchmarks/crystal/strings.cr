# Crystal port of benchmarks/strings.rb, for the README comparison column.

parts = [] of String
100_000.times { |i| parts << "item-#{i}-#{i * 2}" }
joined = parts.join(",")
words = joined.split(",")
puts words.sum { |w| w.size }
puts words.first
