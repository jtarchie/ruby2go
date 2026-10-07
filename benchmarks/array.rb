# rbs_inline: enabled

squares = (1..1_000_000).map { |i| i * i }
evens = squares.select { |i| i.even? }
puts evens.sum
puts evens.size
