# skip: Exception#inspect with an empty message prints `#<ArgumentError: >`; MRI prints just the class name

# rbs_inline: enabled

puts ArgumentError.new("").inspect
begin
  raise KeyError, ""
rescue => e
  puts e.inspect, e.message.inspect
end
