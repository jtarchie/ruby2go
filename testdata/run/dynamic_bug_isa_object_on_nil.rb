# skip: is_a?(Object/BasicObject/Kernel) on a nil T? is false; MRI: nil is an Object, so true

# rbs_inline: enabled

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

none = maybe(-1)
puts none.is_a?(Object).inspect, none.kind_of?(BasicObject).inspect, none.is_a?(Kernel).inspect
puts none.is_a?(Integer).inspect, maybe(2).is_a?(Object).inspect
