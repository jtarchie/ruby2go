# skip: the nil value of puts/print is emitted as a void call, so go build fails ("used as value")

# rbs_inline: enabled

puts puts("a").inspect
puts print("b\n").nil?
