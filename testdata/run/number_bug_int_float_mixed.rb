# skip: Integer op Float coerces the Float literal into Integer (7 / 2.0 is 3, not 3.5); Float#clamp with Integer bounds returns 2.0, not the bound 2
# rbs_inline: enabled

a = 7 #: Integer
puts (a / 2.0).inspect, (1 + 2.0).inspect, (a * 1.0).inspect, (a - 0.0).inspect
x = 2.5 #: Float
puts x.clamp(1, 2).inspect, 0.5.clamp(1, 2).inspect
