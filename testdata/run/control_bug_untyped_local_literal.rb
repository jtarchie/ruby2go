# skip: a local annotated `#: untyped` and initialized from a literal is declared with the literal's Go type (u := Integer(1)), so reassigning another type fails go build

# rbs_inline: enabled

v = 1 #: untyped
puts v.inspect
v = "s"
puts v.inspect
u = false #: untyped
u ||= "was false"
puts u.inspect
