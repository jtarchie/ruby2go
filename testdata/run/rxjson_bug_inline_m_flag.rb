# rbs_inline: enabled

puts "a\nb".match?(/(?m:a.b)/), "a\nb".match?(/(?m)a.b/), "A\nb".match?(/(?mi)a.b/), "xa\nb".match?(/x(?-m:a.b)/m)
