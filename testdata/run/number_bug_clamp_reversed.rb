# skip: Comparable#clamp with min > max returns a bound; MRI raises ArgumentError (min argument must be less than or equal to max argument)
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
