# rbs_inline: enabled

parts = [] #: Array[String]
100_000.times { |i| parts << "item-#{i}-#{i * 2}" }
joined = parts.join(",")
words = joined.split(",")
puts words.sum { |w| w.length }
puts words.first
