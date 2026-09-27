# rbs_inline: enabled

puts "aaa".match(/a{,2}/).inspect, "xbb".match(/xb{,1}/).inspect, ("{,2}" =~ /a{,2}/).inspect
