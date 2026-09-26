# skip: Symbol#inspect quotes :é, :@iv, :@@cv, :$g, :$1 and :` which MRI prints bare

# rbs_inline: enabled

puts :"é".inspect
puts :"@iv".inspect
puts :"@@cv".inspect
puts :"$g".inspect
puts :"$1".inspect
puts :"`".inspect
