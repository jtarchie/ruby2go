# skip: inline (?m) is dot-all in Ruby but multi-line in RE2: /(?m:a.b)/ misses "a\nb" and (?-m:...) inside /.../m stays dot-all

# rbs_inline: enabled

puts "a\nb".match?(/(?m:a.b)/), "a\nb".match?(/(?m)a.b/), "A\nb".match?(/(?mi)a.b/), "xa\nb".match?(/x(?-m:a.b)/m)
