# rbs_inline: enabled

# The unmodified pairing_heap gem: Dijkstra with decrease-key over a weighted grid, then a heapsort.
require "pairing_heap"

SIDE = 150

#: (Integer cell) -> Array[Integer]
def neighbors(cell)
  row, col = cell.divmod(SIDE)
  out = [] #: Array[Integer]
  out << cell - SIDE if row > 0
  out << cell + SIDE if row < SIDE - 1
  out << cell - 1 if col > 0
  out << cell + 1 if col < SIDE - 1
  out
end

dist = { 0 => 0 } #: Hash[Integer, Integer]
queue = PairingHeap::MinPriorityQueue.new
queue.push(0, 0)
until queue.empty?
  top = queue.pop_with_priority #: [Integer, Integer]
  cell, d = top
  neighbors(cell).each do |n|
    alt = d + (cell * 7919 + n * 104_729) % 97 + 1
    known = dist[n]
    next if known && known <= alt

    dist[n] = alt
    queue.include?(n) ? queue.decrease_key(n, alt) : queue.push(n, alt)
  end
end
puts dist.fetch(SIDE * SIDE - 1)
puts dist.values.sum

heap = PairingHeap::SimplePairingHeap.new
100_000.times { |i| heap.push((i * 7919) % 100_003) }
sorted = 0
prev = -1
while heap.any?
  x = heap.pop
  raise "out of order" unless x && x >= prev

  prev = x
  sorted += 1
end
puts sorted
