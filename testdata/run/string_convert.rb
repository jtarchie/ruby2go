# rbs_inline: enabled

# to_i reads an optional sign and leading digits after whitespace; anything else is 0.
puts "42abc".to_i, " 42".to_i, "-7".to_i, "+5".to_i, "abc".to_i, "".to_i, "12e3".to_i
puts "0x1A".to_i, "\n 9 \n".to_i, "- 4".to_i, "--4".to_i, "007".to_i, "-0".to_i, "3.9".to_i
puts "9223372036854775807".to_i, "-9223372036854775808".to_i
puts ("42".to_i + 1).inspect, "12 34".to_i

# to_f on well-formed input, including exponents and leading dots.
puts "3.5".to_f, "abc".to_f, "1e3".to_f, ".5".to_f, "  2.25  ".to_f, "-0.0".to_f
puts "0x1A".to_f, "".to_f, "1.".to_f, "-1.5e-3".to_f, "10".to_f, "1_000.5".to_f
puts ("0.1".to_f + "0.2".to_f).inspect

# ord is the first character's codepoint.
puts "a".ord, "é".ord, "😀".ord, "abc".ord, "\n".ord

# to_sym round-trips through Symbol.
puts "str".to_sym.inspect, "with space".to_sym.inspect, "".to_sym.inspect, "a?".to_sym.inspect
puts ("x".to_sym == :x).inspect, :abc.to_s.to_sym.inspect

# inspect escapes quotes, backslashes, named control chars and #{ #$ #@.
puts "tab\there".inspect, "quote\"d".inspect, "new\nline".inspect, "back\\slash".inspect
puts "\a\b\v\f\r\e".inspect, "'single'".inspect, "".inspect
puts "\#{x} \#$y \#@z #x # #".inspect
puts "é日本😀".inspect, "\u00a0x".inspect, "\u200b".inspect
puts "a\\b\"c'd".inspect.size, "\n".inspect.size
puts ["a\tb", "c\"d"].inspect

# inspect output is itself a valid literal that reads back the same.
src = "x\ty\n\"z\""
puts src.inspect
puts ("x\ty\n\"z\"" == src).inspect
