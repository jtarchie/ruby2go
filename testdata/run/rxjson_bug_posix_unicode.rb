# rbs_inline: enabled

puts "é".match(/[[:alpha:]]/).inspect, "É".match(/[[:upper:]]+/).inspect, "日本1".match(/[[:alnum:]]+/).inspect
puts ("é" =~ /[[:word:]]/).inspect, ("é" =~ /[[:lower:]]/).inspect
puts "٣".match?(/[[:digit:]]/), "\u3000".match?(/[[:space:]]/), "x\u3000".match?(/x[[:^space:]]/)
