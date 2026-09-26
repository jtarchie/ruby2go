# rbs_inline: enabled

strs = ["é", "a\"b", "tab\there", "new\nline", "", "日本語", "\\", "\e", "#{1 + 1}"] #: Array[String]
puts strs.inspect
puts strs.to_s
puts strs.join("|").inspect, strs.join.inspect
puts ["a", "b"].join.inspect, ["a"].join(", ").inspect, ["a", "b"].join("").inspect, ["a", "b"].join("→").inspect

nada = [] #: Array[String]
puts nada.join(",").inspect, nada.inspect, nada.to_s, nada.to_s.inspect

floats = [1.0, 2.5, -0.0, 1e20, 1.0e-5, 100.0, 3.14159, 1e16, 123456789.123] #: Array[Float]
puts floats.inspect, floats.join(" ")
puts [0.1 + 0.2, 1.0 / 3].inspect

ints = [0, -1, 42, 1_000_000, 9223372036854775807, -9223372036854775808] #: Array[Integer]
puts ints.inspect, ints.join(",")

syms = [:a, :"b c", :c?, :d!, :e=, :+] #: Array[Symbol]
puts syms.inspect, syms.join("-")

bools = [true, false, true] #: Array[bool]
puts bools.inspect, bools.join(",")

nested = [[1, 2], [], [3, [4, 5].size]] #: Array[Array[Integer]]
puts nested.inspect, nested.to_s, nested[0].inspect, nested[1].inspect, nested[5].inspect
deep = [[["x"]], [[]]] #: Array[Array[Array[String]]]
puts deep.inspect

maps = [{ a: 1 }, { "b c" => [2] }, {}] #: Array[Hash[untyped, untyped]]
puts maps.inspect

holes = [nil, nil]
puts holes.inspect, holes.size, holes.to_s

puts "interp #{[1, "two", :three, nil, 4.0]}"
puts "empty #{[]}"

puts [1, 2]
puts [[1, 2], [3]]
puts [nil]
print [1, 2], "\n"
print "a", "b", 1, 2.5, nil, :c, "\n"

puts [1, 2] == [1, 2], [1, 2] == [2, 1], [1, 2] == [1, 2, 3], [1, 2, 3] == [1, 2]
puts nested == nested.dup, nested == [[1, 2], [3, 2]], nested == [[1, 2], [7], [3, 2]], [[1], [2]] == [[1], [2]]
puts ints == ints.dup, strs == strs.reverse.reverse, floats == [1.0]
puts ([1] == "1").inspect, (["a"] == ["a"]).inspect, (["a"] == ["A"]).inspect
puts [1.5, 2.0], [:a, :b], [true, false]
print [[1], [2, [3]]], "\n"
multi = [
  1,
  2, # two
  3,
] #: Array[Integer]
puts multi.inspect, "#{[1.5, -0.0]}"
w = %w[]
puts w.inspect, w.size
fl = [3.5, 1.25, 2.0] #: Array[Float]
puts fl.sort.inspect, fl.max.inspect, fl.min_by { |f| -f }.inspect, fl.sort_by { |f| f }.reverse.inspect, fl.include?(2.0)
