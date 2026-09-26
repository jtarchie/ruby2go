# skip: Hash#== compares T? values as *T pointers, so equal contents with an optional value type compare false

# rbs_inline: enabled

a = { "a" => 1, "b" => nil } #: Hash[String, Integer?]
b = { "b" => nil, "a" => 1 } #: Hash[String, Integer?]
puts (a == b).inspect
c = { "s" => "x" } #: Hash[String, String?]
d = { "s" => "x" } #: Hash[String, String?]
puts (c == d).inspect
u1 = { 1 => "x", "k" => nil }
u2 = { "k" => nil, 1 => "x" }
puts (u1 == u2).inspect
