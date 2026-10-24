# Crystal port of benchmarks/array.rb, for the README comparison column.
# Int64 elements: squares overflow Int32 (Crystal raises on overflow, Ruby promotes).

squares = (1_i64..1_000_000_i64).map { |i| i * i }
evens = squares.select { |i| i.even? }
puts evens.sum
puts evens.size
