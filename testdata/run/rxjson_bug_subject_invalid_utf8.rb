# skip: matching a regexp against a String with invalid UTF-8 matches silently; MRI raises ArgumentError "invalid byte sequence in UTF-8"

# rbs_inline: enabled

s = "a\xffb"
begin
  puts s.match?(/b/)
rescue ArgumentError => e
  puts "match? #{e.class}"
end
begin
  puts (s =~ /b/).inspect
rescue ArgumentError => e
  puts "=~ #{e.class}"
end
begin
  puts s.match(/(b)/).inspect
rescue ArgumentError => e
  puts "match #{e.class}"
end
