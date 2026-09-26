# skip: "ab" * -1 panics with a Go error rescued as StandardError; MRI raises ArgumentError (negative argument)

# rbs_inline: enabled

begin
  puts "ab" * -1
rescue ArgumentError => e
  puts "mul: #{e.message}"
end
