# skip: Hash#select/filter/reject yield (key, value) in MRI, so a one-param block gets the key; rb2go yields the [k, v] pair

# rbs_inline: enabled

h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
h.select { |x| puts x.inspect; true }
h.filter { |x| puts x.inspect; false }
h.reject { |x| puts x.inspect; false }
