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
