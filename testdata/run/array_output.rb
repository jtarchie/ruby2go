# rbs_inline: enabled

# Checks whose subject is puts/p formatting of arrays; the rest moved to testdata/test/array_test.rb.

# puts flattens a tuple held as untyped (was array_bug_tuple_as_untyped.rb)
pair = [1, "a"]
puts pair
puts [1, [2, [3]]]

# puts of arrays with nil, empty and nested-empty elements (was array_bugs.rb)
jp_opt = [1, nil, 3] #: Array[Integer?]
puts jp_opt
empty = [] #: Array[Integer]
puts empty
puts [[], [1]]
puts "end"

# splatting a typed array into puts's rest param (was array_bugs.rb)
elem_nums = [1, 2, 3] #: Array[Integer]
puts(*elem_nums)

# map with a void block (puts) collects nils (was array_bugs.rb)
void_nums = [1, 2] #: Array[Integer]
puts void_nums.map { |n| puts n }.inspect

# puts and print of array literals flatten (was array_format.rb)
puts [1, 2]
puts [[1, 2], [3]]
puts [nil]
print [1, 2], "\n"
print "a", "b", 1, 2.5, nil, :c, "\n"
puts [1.5, 2.0], [:a, :b], [true, false]
print [[1], [2, [3]]], "\n"

# puts and print take a rest param of mixed values (was array_splat.rb)
puts 1, 2
puts "a", ["b", "c"], nil, 3
puts
print "x", "y", "\n"
print
puts "end"
