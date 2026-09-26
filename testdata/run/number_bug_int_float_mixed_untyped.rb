# skip: Integer#+ through untyped raises TypeError (no implicit conversion of Float into Integer); MRI gives 6.5
# rbs_inline: enabled

v = 5 #: untyped
w = 1.5 #: untyped
puts (v + 1.5).inspect, (v * w).inspect, (w + v).inspect, (v < w).inspect
