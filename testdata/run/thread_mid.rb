# rbs_inline: enabled

# Thread.new forwards constructor args into the block, as MRI's does.
sum_t = Thread.new(3, 4) { |a, b| puts a + b }
sum_t.join

label_t = Thread.new("x", 2, 3) { |s, a, b| puts "#{s}#{a + b}" }
label_t.join

# join(timeout): a Queue with nothing pushed blocks forever, so the timeout always fires.
blocker = Queue.new #: Queue[Integer]
stuck = Thread.new { blocker.pop }
puts stuck.join(0.01).inspect

# join(timeout) on an already-finished thread always succeeds, no race with its work.
quick = Thread.new { 1 + 1 }
quick.join
puts quick.join(0.01) == quick
puts quick.status

boom_t = Thread.new { raise "boom" }
begin
  boom_t.join
rescue RuntimeError
end
puts boom_t.status.inspect

named = Thread.new { 1 }
puts named.name.inspect
named.name = "worker"
puts named.name
named.join

# ConditionVariable#wait(timeout): nobody signals gate_cv, so this always times out.
gate_m = Mutex.new
gate_cv = ConditionVariable.new
gate_m.synchronize { puts gate_cv.wait(gate_m, 0.01).inspect }

seeded = Queue.new([1, 2, 3]) #: Queue[Integer]
puts seeded.size
puts seeded.pop, seeded.pop, seeded.pop

sq = SizedQueue.new(1) #: SizedQueue[Integer]
puts sq.max
sq.max = 3
puts sq.max
sq.max = 1

# num_waiting: spin (no sleep in this subset) until the second push actually blocks.
sq.push(1)
filler = Thread.new { sq.push(2) }
until sq.num_waiting > 0
end
puts sq.num_waiting
sq.pop
filler.join
puts sq.num_waiting

blocker.close
stuck.join
