# skip: \s does not match \v (RE2's \s is [\t\n\f\r ]; Onigmo's includes \v)

# rbs_inline: enabled

puts ("\v" =~ /\s/).inspect, "a\vb".match?(/a\sb/), "\v".match?(/\S/)
