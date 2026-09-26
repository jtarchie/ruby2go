# skip: #hash on an untyped receiver emits rbHash(...), which is not defined anywhere, so go build fails
# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

v = ident(5)
w = ident("s")
puts (v.hash == 5.hash).inspect, (v.hash == 6.hash).inspect, (w.hash == "s".hash).inspect
