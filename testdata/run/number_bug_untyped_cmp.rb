# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

a = ident(3)
b = ident(4)
puts (a <=> b).inspect, (ident(2.5) <=> ident(2.5)).inspect
puts (a <=> 1).inspect, (ident(1.5) <=> 2.0).inspect
