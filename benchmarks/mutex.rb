# rbs_inline: enabled
# Eight threads bumping one counter under one Mutex: the cost of a contended lock handoff.

N = 8
PER = 300_000
m = Mutex.new
counter = 0
threads = (0...N).map do
  Thread.new do
    PER.times { m.synchronize { counter += 1 } }
  end
end
threads.each(&:join)
puts counter
