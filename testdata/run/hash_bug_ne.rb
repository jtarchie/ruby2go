# rbs_inline: enabled

a = { "x" => 1, "y" => 2 } #: Hash[String, Integer]
b = { "y" => 2, "x" => 1 } #: Hash[String, Integer]
c = { "x" => 1 } #: Hash[String, Integer]
puts (a != b).inspect, (a != c).inspect, (a != a).inspect
