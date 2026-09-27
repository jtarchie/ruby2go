# rbs_inline: enabled

class Foo
end

#: (untyped) -> untyped
def ident(v) = v

puts ident(1).class, ident("s").class.name, ident(Foo.new).class, ident([1]).class, ident(nil).class
puts "#{ident(2.5).class}"
