# rbs_inline: enabled

begin
  puts "".ord
rescue ArgumentError => e
  puts "ord: #{e.message}"
end
