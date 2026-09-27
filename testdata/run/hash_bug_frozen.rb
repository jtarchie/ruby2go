# rbs_inline: enabled

h = { "a" => 1 } #: Hash[String, Integer]
puts h.frozen?.inspect
puts({}.frozen?.inspect)
