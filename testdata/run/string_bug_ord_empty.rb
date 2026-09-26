# skip: "".ord returns 65533; MRI raises ArgumentError (empty string)

# rbs_inline: enabled

begin
  puts "".ord
rescue ArgumentError => e
  puts "ord: #{e.message}"
end
