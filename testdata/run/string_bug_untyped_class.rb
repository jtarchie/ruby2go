# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

puts ident("a").class, ident(:a).class
