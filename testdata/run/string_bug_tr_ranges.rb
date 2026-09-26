# skip: tr treats "a-y" and "^x" literally; MRI supports ranges and negation

# rbs_inline: enabled

puts "hello".tr("a-y", "b-z").inspect
puts "hello".tr("^l", "*").inspect
