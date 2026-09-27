# skip: an ASCII control character inspects as \xNN: strings carry no encoding, and Integer#chr (US-ASCII in MRI) must keep "\x00"; a UTF-8 literal is "\u0000" in MRI

# rbs_inline: enabled

puts "\x00\x01\x7f".inspect
puts "\c?".inspect
puts "\u0085".inspect
puts "\u2028".inspect
puts ["\x00"].inspect
