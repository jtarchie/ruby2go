# rbs_inline: enabled

# Decision 20: calling a method on a nil String? raises NoMethodError, like MRI.
s = "abc"
c = s[5]
begin
  puts c.upcase
rescue NoMethodError
  puts "rescued NoMethodError"
end

# Narrowing makes the call safe; to_s/inspect/nil? are fine on nil itself.
puts c.nil?.inspect, c.to_s.inspect, c.inspect, (c == nil).inspect
d = s[1]
puts d.upcase if d
e = s[-1] || "fallback"
puts e, (s[7] || "fallback")
x = s.index("q")
puts x ? x + 1 : -1

#: (String?) -> String
def label(v)
  return "none" unless v

  v.capitalize
end
puts label(s[0]), label(s[3])

# Uncaught: stdout so far is flushed and the exit status is 1.
puts "before crash"
puts s[9].size
puts "unreachable"
