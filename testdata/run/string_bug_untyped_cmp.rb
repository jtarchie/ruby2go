# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

s = ident("b")
puts (s <=> "a").inspect, (s <=> "b").inspect
y = ident(:b)
puts (y <=> :c).inspect
