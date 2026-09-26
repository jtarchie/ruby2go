# skip: `.class` on nil (a nil literal, or a T? holding nil) raises NoMethodError at run time; MRI returns NilClass

# rbs_inline: enabled

#: (String?) -> String
def kind(s) = s.class.name

puts kind("a")
puts kind(nil)
puts nil.class
