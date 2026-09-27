# rbs_inline: enabled

x = "z"
puts "z".match?(/[#{x}\h]/), "f".match?(/[#{x}\h]/), "]".match?(/[#{x}\h]/), "z]".match(/[#{x}\h]/).inspect
