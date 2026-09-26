# skip: strip/lstrip/rstrip keep NUL; MRI strips "\0" as whitespace on both sides

# rbs_inline: enabled

puts "\0 x \0".strip.inspect
puts "\0 x \0".lstrip.inspect
puts "\0 x \0".rstrip.inspect
