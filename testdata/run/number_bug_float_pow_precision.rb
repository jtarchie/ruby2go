# rbs_inline: enabled

third = 1.0 / 3.0 #: Float
puts (8.0 ** third).inspect, (27.0 ** third).inspect, (1000.0 ** third).inspect
puts (0.1 ** 4.0).inspect, (1.01 ** 3.0).inspect, (9.9 ** 4.0).inspect, (2.0 ** 2.5).inspect, (10.0 ** 2.5).inspect
x = 1.1 #: Float
puts (x ** 10).inspect
neg = -1.1 #: Float
puts (neg ** 3).inspect, ((neg - 1.4) ** -3.0).inspect, (0.5 ** -3.3).inspect, (1e10 ** 30.5).inspect, (1.0000001 ** 1e7).inspect, (7.5 ** -7.5).inspect
