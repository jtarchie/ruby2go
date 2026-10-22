# skip: wontfix: 2 ** -1 is a Rational in MRI; rb2go's Integer#** has a static Integer return type (prelude/integer.rb), so it raises RangeError instead of returning a Rational (decisions 35, 40)
# rbs_inline: enabled

puts (2 ** -1).inspect, (2 ** -2).inspect, (1 ** -5).inspect, (-1 ** -3).inspect
