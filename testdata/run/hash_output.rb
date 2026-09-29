# rbs_inline: enabled

# Checks whose subject is puts/p formatting of hashes; the rest moved to testdata/test/hash_test.rb.

# puts takes a braceless hash (was hash_args.rb)
puts(a: 1)
puts 1, b: 2

# print and puts of hashes and arrays of hashes (was hash_inspect.rb)
h = { "b" => 20, "c" => 30 } #: Hash[String, Integer]
e = {} #: Hash[String, Integer]
print h, "\n"
print e, "|", { a: 1 }, "\n"
puts [h, [{ b: 2 }]]
print [h], "\n"
puts [e]
