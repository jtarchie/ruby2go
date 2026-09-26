# skip: puts of an array holding a typed nil (Array[String?], or [2, nil] inside untyped) panics: the splat keeps a nil *T that `when nil` misses; MRI prints an empty line

# rbs_inline: enabled

b = ["x", nil] #: Array[String?]
puts b
puts "after"
u = [1, [2, nil]] #: untyped
puts u
puts "end"
