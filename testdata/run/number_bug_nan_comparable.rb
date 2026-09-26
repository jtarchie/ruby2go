# skip: Comparable and sorting on a NaN Float return a value; MRI raises ArgumentError (comparison of Float with ... failed) because NaN <=> x is nil
# rbs_inline: enabled

z = 0.0 #: Float
nan = z / z #: Float
begin
  puts nan.between?(0.0, 1.0).inspect
rescue ArgumentError => e
  puts "between? #{e.class}"
end
begin
  puts nan.clamp(0.0, 1.0).inspect
rescue ArgumentError => e
  puts "clamp #{e.class}"
end
begin
  puts [1.0, nan].sort.inspect
rescue ArgumentError => e
  puts "sort #{e.class}"
end
begin
  puts [nan, 1.0].max.inspect
rescue ArgumentError => e
  puts "max #{e.class}"
end
