# skip: Float#<=> with NaN returns -1/1 (cmp.Compare orders NaN first); MRI returns nil
# rbs_inline: enabled

z = 0.0 #: Float
nan = z / z #: Float
puts (nan <=> 1.0).inspect, (1.0 <=> nan).inspect, (nan <=> nan).inspect
