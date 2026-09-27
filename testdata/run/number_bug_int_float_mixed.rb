# rbs_inline: enabled

a = 7 #: Integer
puts (a / 2.0).inspect, (1 + 2.0).inspect, (a * 1.0).inspect, (a - 0.0).inspect
x = 2.5 #: Float
puts x.clamp(1, 2).inspect, 0.5.clamp(1, 2).inspect
