# skip: sort/min/max/sort_by/max_by with nilable keys or elements (counts[w], [counts[w], w], Array[Integer?]) panic in rbCmp (*Integer has no Cmp(*Integer)) even when no key is nil; MRI compares the values

# rbs_inline: enabled
counts = { "a" => 3, "b" => 7, "c" => 5 } #: Hash[String, Integer]
words = ["a", "b", "c"] #: Array[String]
puts words.max_by { |w| counts[w] }.inspect
puts words.min_by { |w| counts[w] }.inspect
puts words.sort_by { |w| [counts[w], w] }.inspect
opt = [3, 1, 2] #: Array[Integer?]
puts opt.sort.inspect, opt.max.inspect
