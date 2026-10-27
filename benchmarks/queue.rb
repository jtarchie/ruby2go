# rbs_inline: enabled
# Four producers and four consumers over one bounded queue, a million small items (decision 403).

P = 4
C = 4
PER = 250_000
q = SizedQueue.new(4096) #: SizedQueue[Integer]
producers = (0...P).map do |i|
  Thread.new do
    PER.times { |j| q << (i * PER + j) }
  end
end
sums = Array.new(C, 0)
counts = Array.new(C, 0)
consumers = (0...C).map do |k|
  Thread.new do
    while (v = q.pop)
      sums[k] += v
      counts[k] += 1
    end
  end
end
producers.each(&:join)
q.close
consumers.each(&:join)
puts sums.sum
puts counts.sum
