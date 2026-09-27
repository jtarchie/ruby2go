# rbs_inline: enabled

puts 200.chr.bytesize.inspect, 255.chr.bytesize.inspect, 128.chr.bytesize.inspect, 127.chr.bytesize.inspect
puts 200.chr.ord.inspect, 255.chr.ord.inspect
begin
  puts 256.chr.bytesize.inspect
rescue RangeError => e
  puts e.message
end
begin
  puts -1.chr.bytesize.inspect
rescue RangeError => e
  puts e.message
end
