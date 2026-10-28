# rbs_inline: enabled

# count, sum and Array.new with a block compile to plain loops with the block's body inside them (decision 402).

W = 8
H = 6
OFFSETS = [-1, 0, 1].freeze

#: (Array[Integer], Integer, Integer) -> Integer
def neighbours(cells, x, y)
  OFFSETS.sum do |dy|
    OFFSETS.count do |dx|
      next false if dx == 0 && dy == 0

      cells[((y + dy) % H) * W + (x + dx) % W] == 1
    end
  end
end

#: (Array[Integer]) -> Array[Integer]
def step(cells)
  Array.new(W * H) do |i|
    n = neighbours(cells, i % W, i / W)
    next 1 if n == 3
    next cells[i].to_i if n == 2

    0
  end
end

cells = Array.new(W * H) { 0 }
[1, W + 2, 2 * W, 2 * W + 1, 2 * W + 2].each { |i| cells[i] = 1 } # a glider
4.times do |gen|
  puts "generation #{gen}: #{cells.count { |c| c == 1 }} alive"
  cells.each_slice(W) { |row| puts row.map { |c| c == 1 ? "#" : "." }.join }
  cells = step(cells)
end

prices = [0.1, 0.2, 0.3, 0.4]
total = prices.sum { |p| p * 3 }
puts "total: #{total}"
puts "exact tenths: #{([0.1] * 10).sum { |x| x }}"
puts "big and small: #{[1e100, 1.0, -1e100].sum { |x| x }}"
squares = Array.new(5) { |i| i * i }
puts "squares: #{squares.inspect}, even: #{squares.count(&:even?)}, sum: #{squares.sum { |s| s }}"
puts(squares.sum { |s| s * 2 })
puts "two big squares" if squares.count { |s| s > 3 } == 2
