# rbs_inline: enabled

h = { "a" => 1 } #: Hash[String, Integer]
ok = h.key?("a")
ok and puts "and ran"
ok && puts("&& ran")
h["a"] && puts("has a")
h["b"] && puts("never")
u = nil #: untyped
u || puts("untyped || ran")
w = 1 #: untyped
w && puts("untyped && ran")
