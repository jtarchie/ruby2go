# skip: .class on an untyped value raises NoMethodError (no DynClass wrapper); MRI gives String and Symbol

# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

puts ident("a").class, ident(:a).class
