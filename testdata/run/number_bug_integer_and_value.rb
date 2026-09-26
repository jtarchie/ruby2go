# skip: `a && 2` with a non-nilable Integer/Float local on the left drops the left operand entirely, so a local used only there fails go build (declared and not used)
# rbs_inline: enabled

a = 1 #: Integer
v = a && 2
puts v.inspect
b = 2.5 #: Float
w = b && 1.0
puts w.inspect
