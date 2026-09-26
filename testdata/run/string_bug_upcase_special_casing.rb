# skip: upcase/capitalize/downcase use Go simple case mapping; MRI applies SpecialCasing (ß -> SS, ﬀ -> FF, İ -> i̇) and titlecase (ǆ -> ǅ)

# rbs_inline: enabled

puts "straße".upcase.inspect
puts "ß".capitalize.inspect
puts "İ".downcase.inspect
puts "ﬀ".upcase.inspect
puts "ǆa".capitalize.inspect
