# skip: Onigmo class set syntax (intersection [a-z&&[^aeiou]], nested [a[bc]]) is passed to RE2, which reads the inner [ as a literal and the second ] as a literal: silently different matches (MRI: "bcd", "b"; rb2go matches "&]" and "b]")

# rbs_inline: enabled

puts "aebcd".match(/[a-z&&[^aeiou]]+/).inspect, "&]".match(/[a-z&&[^aeiou]]/).inspect
puts "b".match?(/[a[bc]]/), "b]".match(/[a[bc]]/).inspect
