# rbs_inline: enabled

puts "ab".match?(/a\u0062/), "é".match?(/\u00e9/), "é".match?(/\u{e9}/), "日本".match?(/\u{65e5 672c}/)
puts "\e[31mred\e[0m".match(/\e\[\d+m/).inspect
