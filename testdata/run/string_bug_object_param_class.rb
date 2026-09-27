# rbs_inline: enabled

#: (Object) -> String
def kind(o) = o.class.name

puts kind("a"), kind(:a)
