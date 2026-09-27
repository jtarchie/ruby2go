# rbs_inline: enabled

puts RuntimeError.new("a\nb").inspect
puts ArgumentError.new("tab\there").inspect
