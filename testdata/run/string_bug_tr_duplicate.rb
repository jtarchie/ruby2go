# skip: tr with a repeated from-character uses the first mapping; MRI uses the last

# rbs_inline: enabled

puts "hello".tr("ll", "xy").inspect
