# rbs_inline: enabled

x = 0 #: Integer
f = 0.0 #: Float
puts "0 is truthy" if x
puts(f ? "0.0 is truthy" : "0.0 is falsy")
n = 3
while n
  n -= 1
  break if n.zero?
end
puts n.inspect
