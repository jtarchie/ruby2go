# rbs_inline: enabled

m = /(a)(b)?/.match("a")
puts m.captures.compact.size if m
found = ["a1", "b", "c3"].map { |s| s.match(/\d/) }.compact
puts found.size
