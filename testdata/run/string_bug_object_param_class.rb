# skip: an Object-typed value is Go any and dispatches as untyped, and .class on untyped raises NoMethodError (no DynClass wrapper, see dynamic_bug_untyped_class); MRI gives String/Symbol

# rbs_inline: enabled

#: (Object) -> String
def kind(o) = o.class.name

puts kind("a"), kind(:a)
