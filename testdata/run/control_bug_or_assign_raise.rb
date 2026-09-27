# rbs_inline: enabled

y = 5 #: Integer?
y ||= raise(ArgumentError, "never")
puts y
x = nil #: Integer?
begin
  x ||= raise(KeyError, "x missing")
  puts "unreachable #{x}"
rescue KeyError => e
  puts e.message
end
