# rbs_inline: enabled

# Decision 34: symbol keys print as labels when they are identifiers, quoted otherwise.
syms = { :+ => 1, :"é" => 4, :A => 5, :_ => 6, :"9a" => 7, :"a-b" => 8, :"" => 9, :"a!" => 10, :"A?" => 11, :@iv => 12, :$g => 13, :"a\nb" => 14, :[] => 15, :"日本" => 16, :"foo=" => 17, :"Foo=" => 18, :nil => 19, :if => 20, :__ => 21, :"a?b" => 22, :"ǅ" => 24, :"a٣" => 26, :"a b" => 27 } #: Hash[Symbol, Integer]
puts syms.inspect
puts({ port: 1, "a b": 2, :+ => 3, ok?: 4, "s" => 5, "set=": 6 }.inspect)

# String keys and values go through String#inspect escaping.
strs = { "héllo" => "wörld", "日本" => "語", "😀" => "", "tab\there" => "q\"uote", "nl\n" => "back\\slash", "" => "\e", "#" => "\#{x}" } #: Hash[String, String]
puts strs.inspect

puts({ "f" => 1.5, "g" => 100.0, "h" => 1.0e-5, "i" => -0.0, "j" => 1e20, "k" => 123456789.125 }.inspect)
puts({ 3 => "c", -1 => "neg", 0 => "zero", 4_611_686_018_427_387_904 => "big" }.inspect)
puts({ true => "t", false => "f" }.inspect)
puts({ 0.5 => :half, -2.0 => :neg }.inspect)

e = {} #: Hash[String, Integer]
puts e.inspect, e.to_s
puts({ "x" => { "y" => { "z" => {} } }, "e" => {} }.inspect)
puts({ "list" => [1, [2, []]], "empty" => [] }.inspect)
puts [{ "a" => 1 }, {}, { b: :c }].inspect

# Mixed keys make Hash[untyped, untyped]; each key keeps its own inspect.
mixed = { 1 => :a, 1.0 => :b, nil => 3, true => 4, [1, 2] => 5, "s" => 6, s: 7 }
puts mixed.inspect, mixed.size
puts({ port: 8080, host: "h", on: true, none: nil, list: [1, 2], sub: { k: "v" } }.inspect)

h = { "b" => 20, "c" => 30 } #: Hash[String, Integer]
puts h.to_s
puts h
puts "interp: #{h} and #{e}"
print h, "\n"
print e, "|", { a: 1 }, "\n"
puts h.inspect.size
puts h.inspect == h.to_s

# Values keep their own inspect: quoted symbols, nil, nested arrays.
puts({ a: :"b c", b: :+, c: :"9", d: :ok? }.inspect)
puts({ :"a\"b" => 1, :"\\" => 2, :"a'b" => 3 }.inspect)
puts({ "x" => nil }.inspect, { nil => nil }.inspect)
puts({ "t" => [1, "a", :s, nil, 1.5, true] }.inspect)

# puts flattens arrays but prints each hash through to_s (= inspect).
puts [h, [{ b: 2 }]]
puts [h].to_s
print [h], "\n"
puts "#{[h]} #{{ x: [h] }}"
puts [e]
