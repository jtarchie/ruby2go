# skip: Exception#inspect with a newline in the message prints it raw; Ruby 4 prints `#<RuntimeError:"a\nb">`

# rbs_inline: enabled

puts RuntimeError.new("a\nb").inspect
puts ArgumentError.new("tab\there").inspect
