# skip: to_i stops at "_"; MRI reads 1_000 as 1000

# rbs_inline: enabled

puts "1_000".to_i
puts "12_345xyz".to_i
