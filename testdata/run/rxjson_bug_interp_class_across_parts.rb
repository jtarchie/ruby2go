# skip: translateRegexp restarts its character-class state for each static part of an interpolated regexp, so /[#{x}\h]/ expands \h to a nested [0-9a-fA-F] inside the open class and silently matches "z]" instead of "z"

# rbs_inline: enabled

x = "z"
puts "z".match?(/[#{x}\h]/), "f".match?(/[#{x}\h]/), "]".match?(/[#{x}\h]/), "z]".match(/[#{x}\h]/).inspect
