# skip: needs string-inspect-escapes: strip now drops NUL, but inspect prints the remaining NUL as \x00; MRI prints \u0000

# rbs_inline: enabled

puts "\0 x \0".strip.inspect
puts "\0 x \0".lstrip.inspect
puts "\0 x \0".rstrip.inspect
