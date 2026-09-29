# rbs_inline: enabled

# puts/print formatting of numbers, bools and nil T? values; the values themselves are checked in testdata/test/number_test.rb.
t = true #: bool
f = false #: bool
puts t, f
print t, f, "\n"

h = { "a" => 1 } #: Hash[String, Integer]
fh = { "pi" => 3.14 } #: Hash[String, Float]
bh = { "on" => true } #: Hash[String, bool]
x = h["zz"]
y = fh["zz"]
z = bh["zz"]
puts x, y, z
print x, y, z, "\n"

fx = 2.5 #: Float
fy = -1.25 #: Float
fz = 0.0 #: Float
puts fx, fy, fz
print fx, fy, "\n"

a = 7 #: Integer
b = -7 #: Integer
iz = 0 #: Integer
puts a, b, iz
print a, b, iz, "\n"
