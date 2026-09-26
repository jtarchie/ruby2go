# skip: tr with an empty to-list panics (index out of range); MRI deletes the characters

# rbs_inline: enabled

puts "hello".tr("l", "").inspect
