# skip: `&.` on an untyped nil (or an untyped? such as Hash[String, untyped]#[] holding nil) calls the method and raises NoMethodError; MRI returns nil

# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

u = ident(nil)
puts u&.size.inspect
v = ident("abc")
puts v&.size.inspect
h = { "n" => nil, "s" => "str" } #: Hash[String, untyped]
puts h["s"]&.size.inspect, h["zz"]&.size.inspect
puts h["n"]&.size.inspect
a = [nil, "x"] #: Array[untyped]
puts a[0]&.size.inspect
