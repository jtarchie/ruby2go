# rbs_inline: enabled

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

maybes = [maybe(1), maybe(2)] #: Array[Integer?]
puts (maybes == [maybe(1), maybe(2)]).inspect
puts maybes.include?(maybe(2)).inspect
puts maybes.include?(2).inspect
