# rbs_inline: enabled

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

#: (untyped?) -> String
def show(v) = v.inspect

puts show(maybe(-1)), show(maybe(4))
