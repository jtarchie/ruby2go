# skip: POSIX bracket classes are ASCII-only on RE2 ([[:alpha:]] misses "é", [[:digit:]] misses "٣", [[:space:]] misses U+3000); Onigmo's are Unicode

# rbs_inline: enabled

puts "é".match(/[[:alpha:]]/).inspect, "É".match(/[[:upper:]]+/).inspect, "日本1".match(/[[:alnum:]]+/).inspect
puts ("é" =~ /[[:word:]]/).inspect, ("é" =~ /[[:lower:]]/).inspect
puts "٣".match?(/[[:digit:]]/), "\u3000".match?(/[[:space:]]/), "x\u3000".match?(/x[[:^space:]]/)
