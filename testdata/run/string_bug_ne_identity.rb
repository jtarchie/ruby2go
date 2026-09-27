# rbs_inline: enabled

ch = "abc".chars[1]
puts (ch != "b").inspect
built = "a" + "b"
puts (built != "ab").inspect, ("x".upcase != "X").inspect, ("ab".dup != "ab").inspect
n = 0
"abc".each_char { |c| n += 1 if c != "b" }
puts n
puts ("b" != "abc"[1]).inspect
