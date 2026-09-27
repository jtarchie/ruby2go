# rbs_inline: enabled

h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
h.each_pair { |x| puts x.inspect }
