# rbs_inline: enabled

h = { "a" => 1 } #: Hash[String, Integer]
ok = h.key?("zz")
ok or puts "or ran"
h["b"] || puts("no b")
h["a"] || puts("never")
