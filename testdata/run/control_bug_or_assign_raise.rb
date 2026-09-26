# skip: `x ||= raise(...)` assigns the raise's `panic(...)` as a value, so go build fails ((no value) used as value); `x || raise(...)` works

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
