# skip: strip, to_i and to_f skip Unicode spaces (U+00A0, U+3000) through strings.TrimSpace; MRI trims only ASCII whitespace and NUL

# rbs_inline: enabled

puts "\u00a0x\u00a0".strip.inspect
puts "\u3000x".strip.inspect
puts "\u00a012".to_i, "\u300012".to_i
puts "\u00a01.5".to_f
