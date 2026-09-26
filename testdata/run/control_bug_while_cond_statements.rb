# skip: a while/until condition that needs statements (an assignment like `while (x = q.shift)`, or a lifted `||`) is evaluated once before the loop, so it never changes

# rbs_inline: enabled

queue = [1, 2, 3] #: Array[Integer]
guard = 0
while (item = queue.shift)
  guard += 1
  break if guard > 5
  puts item
end
puts queue.inspect, guard

q = [1, 2, 5, 1] #: Array[Integer]
guard = 0
while (q[0] || 0) < 3
  guard += 1
  break if guard > 5
  puts q.shift.inspect
end
puts guard

stack = [4, 0] #: Array[Integer]
guard = 0
until (top = stack.pop).nil?
  guard += 1
  break if guard > 5
  puts top
end
puts guard
