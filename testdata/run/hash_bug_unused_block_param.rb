# rbs_inline: enabled

h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
puts h.select { |k, v| v > 1 }.inspect
puts h.map { |k, v| "#{k}=#{v}" }.inspect
puts h.find { |k, v| v > 1 }.inspect
puts h.min_by { |k, v| k.size }.inspect
