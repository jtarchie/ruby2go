# skip: String#inspect writes control characters as \xNN and prints U+0085/U+2028 raw; MRI writes \u0000, \u007F, \u0085, \u2028

# rbs_inline: enabled

puts "\x00\x01\x7f".inspect
puts "\c?".inspect
puts "\u0085".inspect
puts "\u2028".inspect
puts ["\x00"].inspect
