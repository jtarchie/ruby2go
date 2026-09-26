# skip: to_f runs strconv.ParseFloat on the whole string: trailing text gives 0.0, and Go-only syntax (Infinity, NaN, inf, 0x1p3) parses; MRI reads the Ruby numeric prefix only

# rbs_inline: enabled

puts "3.5abc".to_f
puts "1.5 kg".to_f
puts "1e".to_f, "1.5.3".to_f, "1__0".to_f, "  +1.5e2x".to_f
puts "Infinity".to_f
puts "NaN".to_f
puts "inf".to_f, "0x1p3".to_f
