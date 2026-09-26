# skip: Ruby's \uXXXX, \u{X} and \e regexp escapes are rejected at transpile time ("invalid escape sequence"); translateRegexp could rewrite them to \x{...}

# rbs_inline: enabled

puts "ab".match?(/a\u0062/), "é".match?(/\u00e9/), "é".match?(/\u{e9}/), "日本".match?(/\u{65e5 672c}/)
puts "\e[31mred\e[0m".match(/\e\[\d+m/).inspect
