# rbs_inline: enabled

nk = { nil => 1, "a" => 2 } #: Hash[String?, Integer]
puts nk[nil].inspect, nk["a"].inspect, nk.key?("a").inspect
s = "b"
nk[s] = 3
puts nk["b"].inspect, nk.size
nk["b"] = 4
puts nk.size
# group_by/tally keyed by optional values.
h = { "a" => 1, "b" => 1, "c" => nil, "d" => nil } #: Hash[String, Integer?]
puts h.group_by { |_k, v| v }.size
puts h.values.tally.size
