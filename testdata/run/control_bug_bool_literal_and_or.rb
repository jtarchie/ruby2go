# rbs_inline: enabled

puts (false && "never").inspect
puts (true || "never").inspect
puts (true && "yes").inspect
puts (false || "fallback").inspect
