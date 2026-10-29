# rbs_inline: enabled

# Twenty thousand short-lived threads: what starting and joining one costs (decision 403).

BATCHES = 200
PER = 100
total = 0
BATCHES.times do |b|
  ts = (0...PER).map { |i| Thread.new(b * PER + i) { |x| x * 2 } }
  total += ts.sum { |t| t.value }
end
puts total
