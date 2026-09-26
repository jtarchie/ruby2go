# skip: Array#compact keeps nil elements of an Array[T?] (captures.compact.size is 2, MRI 1; map { match }.compact keeps the failed match)

# rbs_inline: enabled

m = /(a)(b)?/.match("a")
puts m.captures.compact.size if m
found = ["a1", "b", "c3"].map { |s| s.match(/\d/) }.compact
puts found.size
