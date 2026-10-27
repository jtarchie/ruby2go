# rbs_inline: enabled
# A Fiber resumed and yielded 300k times, then an external Enumerator pulled as often: the cost of a coroutine switch (decisions 140, 403).

N = 300_000
f = Fiber.new do
  i = 0
  while i < N
    Fiber.yield i
    i += 1
  end
  -1
end
s = 0
N.times { s += f.resume }
puts s
e = Enumerator.new do |y|
  i = 0
  while i < N
    y << i
    i += 1
  end
end
t = 0
N.times { t += e.next }
puts t
