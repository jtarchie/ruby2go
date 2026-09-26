# skip: .class on an Object-typed value emits o._ClassOf() on Go any, which go build rejects; MRI gives String/Symbol

# rbs_inline: enabled

#: (Object) -> String
def kind(o) = o.class.name

puts kind("a"), kind(:a)
