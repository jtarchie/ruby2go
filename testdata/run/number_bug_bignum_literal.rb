# skip: wontfix: an Integer literal beyond 64 bits is a compile error (README decision 35); MRI prints the Bignum
# rbs_inline: enabled

puts 9_223_372_036_854_775_808.inspect
puts 123_456_789_012_345_678_901_234_567_890.to_s
