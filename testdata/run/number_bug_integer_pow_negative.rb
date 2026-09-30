# skip: wontfix: 2 ** -1 is a Rational in MRI; rb2go has none and raises RangeError (docs/design.md decision 35)
# rbs_inline: enabled

puts (2 ** -1).inspect, (2 ** -2).inspect, (1 ** -5).inspect, (-1 ** -3).inspect
