# skip: join and puts on an Array[Integer?] holding nil panic (typed nil *Integer reaches rbToS)

# rbs_inline: enabled
opt = [1, nil, 3] #: Array[Integer?]
puts opt.join("-").inspect
puts opt
