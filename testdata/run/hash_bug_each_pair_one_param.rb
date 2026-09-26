# skip: Hash#each_pair yields (key, value) to a one-param block in rb2go, so |x| gets the key; MRI's each_pair, like each, gives an arity-1 block the [k, v] pair

# rbs_inline: enabled

h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
h.each_pair { |x| puts x.inspect }
