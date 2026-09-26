# skip: Comparable#clamp with min > max returns min; MRI raises ArgumentError

# rbs_inline: enabled

begin
  puts "b".clamp("c", "a")
rescue ArgumentError => e
  puts "clamp: #{e.message}"
end
