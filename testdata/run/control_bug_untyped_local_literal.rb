# rbs_inline: enabled

v = 1 #: untyped
puts v.inspect
v = "s"
puts v.inspect
u = false #: untyped
u ||= "was false"
puts u.inspect
