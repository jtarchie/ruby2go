# rbs_inline: enabled

begin
  puts "b".clamp("c", "a")
rescue ArgumentError => e
  puts "clamp: #{e.message}"
end
