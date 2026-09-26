# skip: `.class` on an untyped value is a dynamic call no class answers, so it raises NoMethodError; MRI returns the class (Integer, String, Foo, Array, NilClass, Float)

# rbs_inline: enabled

class Foo
end

#: (untyped) -> untyped
def ident(v) = v

puts ident(1).class, ident("s").class.name, ident(Foo.new).class, ident([1]).class, ident(nil).class
puts "#{ident(2.5).class}"
