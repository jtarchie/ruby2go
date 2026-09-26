# rbs_inline: enabled

# a Go nil dereference inside a closure block must surface as an uncaught NoMethodError: exit 1
none = nil #: Integer?
puts "before"
xs = [1, 2] #: Array[Integer]
ys = xs.map { |x| none + x }
puts ys.inspect
