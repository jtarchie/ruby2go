# skip: a parenthesised expression whose value is a numeric literal loses its Integer()/Float() conversion when used as a receiver, so go build fails ((-2).Pow undefined on untyped int)
# rbs_inline: enabled

puts ((-2) ** 3).inspect
puts (3).inspect, (2.5).inspect, (-0).inspect
puts (7).even?.inspect, (1.5).floor.inspect
a = 1 #: Integer
puts (a && 2).inspect, a.inspect
