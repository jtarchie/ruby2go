# skip: Float#** uses Go math.Pow, which is not correctly rounded like libm pow(): 8.0 ** (1.0 / 3.0) is 1.9999999999999998 (MRI 2.0), 0.1 ** 4.0 is 0.00010000000000000005 (MRI 0.00010000000000000002)
# rbs_inline: enabled

third = 1.0 / 3.0 #: Float
puts (8.0 ** third).inspect, (27.0 ** third).inspect, (1000.0 ** third).inspect
puts (0.1 ** 4.0).inspect, (1.01 ** 3.0).inspect, (9.9 ** 4.0).inspect, (2.0 ** 2.5).inspect, (10.0 ** 2.5).inspect
x = 1.1 #: Float
puts (x ** 10).inspect
