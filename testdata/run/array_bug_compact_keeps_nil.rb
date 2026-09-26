# skip: compact on Array[Integer?] keeps nils (any(x) != nil is true for a typed nil pointer)

# rbs_inline: enabled
opt = [1, nil, 3, nil] #: Array[Integer?]
puts opt.compact.size
opt.compact.each { |x| puts x.inspect }
