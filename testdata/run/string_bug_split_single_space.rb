# skip: split(" ") splits on each space; MRI treats " " like no separator (awk mode)

# rbs_inline: enabled

puts "a  b ".split(" ").inspect
puts " a b".split(" ").inspect
