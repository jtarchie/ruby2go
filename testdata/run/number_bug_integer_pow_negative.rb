# skip: Integer#** with a negative exponent returns 1 (the loop never runs); MRI returns a Rational ((1/2)), which rb2go cannot represent
# rbs_inline: enabled

puts (2 ** -1).inspect, (2 ** -2).inspect, (1 ** -5).inspect, (-1 ** -3).inspect
