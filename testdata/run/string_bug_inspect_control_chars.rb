# skip: blocked: strings carry no encoding, so an ASCII control character inspects as \xNN (Integer#chr, US-ASCII in MRI, must keep "\x00") where MRI prints a UTF-8 literal's as \uNNNN ("\u0000")

# rbs_inline: enabled

puts "\x00\x01\x7f".inspect
puts "\c?".inspect
puts "\u0085".inspect
puts "\u2028".inspect
puts ["\x00"].inspect
