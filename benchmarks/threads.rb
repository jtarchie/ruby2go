# rbs_inline: enabled
# Eight CPU-bound threads with no shared state: how far threads scale without a GVL (decision 26).

N = 8
ITER = 1_000_000
out = Array.new(N, 0)
threads = (0...N).map do |i|
  Thread.new do
    s = 0
    j = 0
    while j < ITER
      s = (s + ((j ^ i) & 1023)) % 1_000_003
      j += 1
    end
    out[i] = s
  end
end
threads.each(&:join)
puts out.inspect
