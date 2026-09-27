# rbs_inline: enabled

begin
  puts "ab" * -1
rescue ArgumentError => e
  puts "mul: #{e.message}"
end
