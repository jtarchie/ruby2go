# skip: a literal nil on the left of `||` is rejected at compile time (`||` on a non-nilable nil is always the left side); Ruby returns the right side

# rbs_inline: enabled

puts (nil || false).inspect
puts (nil || "right").inspect
