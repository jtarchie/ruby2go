# skip: Kernel#frozen? is always true; a Hash literal is not frozen in MRI

# rbs_inline: enabled

h = { "a" => 1 } #: Hash[String, Integer]
puts h.frozen?.inspect
puts({}.frozen?.inspect)
