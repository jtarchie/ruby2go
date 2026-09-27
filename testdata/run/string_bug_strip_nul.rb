# skip: blocked: by string_bug_inspect_control_chars; strip drops NUL as MRI does, but inspect prints the remaining NUL as \x00 where MRI prints \u0000

# rbs_inline: enabled

puts "\0 x \0".strip.inspect
puts "\0 x \0".lstrip.inspect
puts "\0 x \0".rstrip.inspect
