# rbs_inline: enabled

begin
  puts 5.clamp(10, 1).inspect
rescue ArgumentError => e
  puts e.message
end
begin
  puts 2.5.clamp(3.0, 1.0).inspect
rescue ArgumentError => e
  puts e.message
end
