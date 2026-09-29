# rbs_inline: enabled

# puts of a Regexp or MatchData prints its to_s (the rest of rxjson_* lives in testdata/test/rxjson_test.rb).
puts(/a/)
m2 = "key=value; other=x".match(/(\w+)=(\w+)/)
puts m2 if m2
while (wm = "abc".match(/c/))
  puts wm
  break
end
