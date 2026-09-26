# skip: <=> on an untyped value compiles to rbCmp(any, String("a")), which go build rejects (type String does not match inferred type any for T)

# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

s = ident("b")
puts (s <=> "a").inspect, (s <=> "b").inspect
y = ident(:b)
puts (y <=> :c).inspect
