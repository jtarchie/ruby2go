# Crystal port of benchmarks/math.rb, for the README comparison column.

total = 0.0
200_000.times { |i| total += Math.sin(i.to_f) * Math.cos(i.to_f) }
puts total.round(4)
