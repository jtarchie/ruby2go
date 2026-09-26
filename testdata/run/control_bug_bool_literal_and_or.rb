# skip: a literal true/false on the left of `&&`/`||` with a non-boolean right side stores a Go bool in the untyped result, so inspect panics (bool is not I_Inspect)

# rbs_inline: enabled

puts (false && "never").inspect
puts (true || "never").inspect
puts (true && "yes").inspect
puts (false || "fallback").inspect
