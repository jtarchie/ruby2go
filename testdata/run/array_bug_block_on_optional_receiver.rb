# skip: a block call on a possibly-nil Array (Hash#[] result) loses its block: "map requires a block" / "each is an iterator ... call it with a block"

# rbs_inline: enabled
h = { "a" => [1, 2] } #: Hash[String, Array[Integer]]
puts h["a"].map { |x| x * 2 }.inspect
h["a"].each { |x| puts x }
