# rbs_inline: enabled
opt = [1, nil, 3] #: Array[Integer?]
puts opt.inspect
puts opt.to_s
puts "interp #{opt}"
tail = [1, nil] #: Array[Integer?]
puts tail.last.inspect
puts tail.pop.inspect
lit = [4, nil]
puts lit.inspect
