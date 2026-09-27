# rbs_inline: enabled
h = { "a" => [1, 2] } #: Hash[String, Array[Integer]]
puts h["a"].map { |x| x * 2 }.inspect
h["a"].each { |x| puts x }
