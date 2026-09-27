# rbs_inline: enabled

#: (String?) -> String
def kind(s) = s.class.name

puts kind("a")
puts kind(nil)
puts nil.class
