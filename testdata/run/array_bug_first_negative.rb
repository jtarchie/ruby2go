# skip: first(-1) returns [] instead of raising ArgumentError (negative array size)

# rbs_inline: enabled
nums = [1, 2] #: Array[Integer]
begin
  nums.first(-1)
  puts "no error"
rescue ArgumentError => e
  puts e.message
end
puts nums.take(-1).inspect
