# rbs_inline: enabled

# Eight threads drawing from Kernel#rand at once: a shared generator serializes them unless the program never seeds (decision 403).

N = 8
PER = 200_000
acc = Array.new(N, 0)
threads = (0...N).map do |i|
  Thread.new do
    h = 0
    PER.times { h ^= rand(1000) }
    acc[i] = h
  end
end
threads.each(&:join)
puts acc.all? { |h| h < 1024 }
