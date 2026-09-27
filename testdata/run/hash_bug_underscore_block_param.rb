# rbs_inline: enabled

h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
h.each { |_, v| puts v }
puts h.map { |_, v| v * 3 }.inspect
puts h.select { |k, _| k.size > 1 }.inspect
n = 0
h.each_key { |_| n += 1 }
puts n
