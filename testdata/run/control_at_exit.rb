# at_exit handlers run LIFO after main; one registered inside a handler runs next.
# `exit n` in a handler replaces the status; the uncaught error still prints last.
at_exit { puts "first registered, runs last" }
at_exit do
  puts "second"
  at_exit { puts "nested, runs next" }
end
at_exit do
  puts "third"
  exit 5
end
at_exit { $stderr.puts "stderr handler" }

def boom
  raise ArgumentError, "main failed"
end

puts "main"
boom
puts "unreachable"
