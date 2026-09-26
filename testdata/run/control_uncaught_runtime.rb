# rbs_inline: enabled

# an uncaught Go runtime panic (integer divide by zero) still runs ensure and exits 1, like MRI's ZeroDivisionError

#: (Integer) -> Integer
def div(x)
  10 / x
ensure
  puts "div ensure"
end
puts "before"
puts div(2)
puts div(0)
puts "unreachable"
