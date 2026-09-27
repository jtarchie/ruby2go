# rbs_inline: enabled

scores = { "b" => 2, "a" => 1, "c" => 3 } #: Hash[String, untyped]
puts scores.max_by { |_k, v| v }.inspect
puts scores.min_by { |_k, v| v }.inspect
puts scores.sort_by { |_k, v| v }.inspect
# Untyped keys: the pair comparison reaches rbCmp on the keys.
byk = { 2 => "b", 1 => "a" } #: Hash[untyped, String]
puts byk.sort.inspect, byk.min.inspect
# Sorting option hashes by a field is the same comparison.
people = [{ name: "a", age: 30 }, { name: "b", age: 20 }] #: Array[Hash[Symbol, untyped]]
puts people.min_by { |p| p.fetch(:age) }.inspect
