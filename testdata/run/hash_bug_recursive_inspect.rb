# skip: a Hash containing itself prints {...} in MRI; rb2go's inspect recurses until the Go stack overflows (exit 2, stdout lost)

# rbs_inline: enabled

h = {} #: Hash[String, untyped]
h["me"] = h
h["n"] = 1
puts h.size
puts h.inspect
