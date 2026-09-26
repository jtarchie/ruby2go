# skip: an Integer literal beyond int64 compiles, then go build fails (constant overflows Integer); MRI prints it
# rbs_inline: enabled

puts 9_223_372_036_854_775_808.inspect
puts 123_456_789_012_345_678_901_234_567_890.to_s
