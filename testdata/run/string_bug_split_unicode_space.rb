# skip: split with no separator splits on U+00A0/U+3000 (strings.Fields); MRI splits on ASCII whitespace only

# rbs_inline: enabled

puts "a\u00a0b\u3000c d".split.inspect
puts "x\u00a0y".split(nil).size
