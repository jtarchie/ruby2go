# skip: {,n} is Ruby's {0,n} but RE2 reads it as literal text, so /a{,2}/ does not match "aaa"

# rbs_inline: enabled

puts "aaa".match(/a{,2}/).inspect, "xbb".match(/xb{,1}/).inspect, ("{,2}" =~ /a{,2}/).inspect
