# frozen_string_literal: true

# rbs_inline: enabled

# Strings stored in collections and passed through blocks, &:sym and chained calls.
words = %w[pear Apple fig banana kiwi] #: Array[String]
puts words.map(&:upcase).inspect, words.select { |w| w.start_with?("f") }.inspect
puts words.sort_by(&:size).inspect, words.max_by(&:length).inspect, words.min_by { |w| w.downcase }.inspect
puts words.group_by(&:size).inspect, words.sort.inspect, words.sort_by(&:downcase).inspect
puts words.join(", "), words.map { |w| w[0] }.inspect, words.map(&:reverse).map(&:capitalize).inspect
puts words.include?("kiwi").inspect, words.include?("Kiwi").inspect, words.include?("ki" + "wi").inspect
puts words.tally.inspect, words.uniq.size, words.reject(&:empty?).size, words.any? { |w| w.end_with?("i") }
puts words.inject("") { |a, b| a + "|" + b }, words.max.inspect, words.min.inspect, words.find { |w| w.size == 3 }.inspect
puts words.map(&:to_sym).inspect, words.map(&:inspect).join(" "), words.flat_map(&:chars).size
words.each_with_index { |w, i| print i, w, " " }
puts

# String? elements: || and && narrow each one.
opt = ["a", nil, "c"] #: Array[String?]
puts opt.map { |x| x || "-" }.join, opt.map { |x| x && x.upcase }.map { |y| y || "nil" }.join(",")

# Rebinding with += / *= / ||= builds new strings (frozen literals are never mutated).
acc = ""
words.each { |w| acc += w[0] || "" }
puts acc
acc2 = "x"
acc2 += "y"
acc2 *= 2
puts acc2
name = nil #: String?
name ||= "default"
puts name
name ||= "other"
puts name

# &. on String?, and a narrowed element from a sub-array.
c = "abc"[9]
puts c&.upcase.inspect, (c&.size || -1), "abc"[0]&.upcase.inspect
first = words.first(1)[0]
puts first.upcase if first

# Hash[String, Array[String]] built by rebinding, and String keys by value.
cache = {} #: Hash[String, Array[String]]
words.each do |w|
  k = w[0] || ""
  cache[k] = (cache[k] || []) + [w]
end
puts cache.inspect, cache["p"].inspect, cache["p" + ""].inspect, cache.key?("z").inspect
lens = {} #: Hash[String, Integer]
words.each { |w| lens[w.downcase] = w.size }
puts lens.inspect, lens.keys.map(&:upcase).inspect, lens.map { |k, v| "#{k}=#{v}" }.join("&")

# Strings through then/nested blocks and a method that yields from each_char.

#: (String) { (String) -> void } -> void
def each_vowel(s)
  s.each_char { |ch| yield ch if "aeiou".include?(ch) }
end
each_vowel("education") { |v| print v.upcase }
puts
puts "x".then { |s| s * 3 }.then { |s| s.center(7) }.inspect
nested = [["b", "a"], ["d", "c"]] #: Array[Array[String]]
puts nested.map { |pair| pair.sort.join }.inspect, nested.map { |pair| pair[0] }.inspect

# split into multiple assignment (a missing part is nil), line-oriented chains, tuple sort keys.
k, v = "key=val".split("=")
puts k, v.inspect
solo, rest = "solo".split(",")
puts solo.inspect, rest.inspect
text = "  one \n two\n\n three  \n"
puts text.lines.map(&:chomp).inspect, text.split("\n").map(&:strip).reject(&:empty?).inspect
puts words.sort_by { |w| [-w.size, w] }.inspect, words.map { |w| w.clamp("b", "g") }.inspect
puts words.select { |w| w.between?("b", "g") }.inspect
fields = "name: Ann, age: 30".split(", ").map { |f| f.split(": ") }
puts fields.inspect, fields.map { |f| f[1] }.inspect
parts = [] #: Array[String]
"a,b;c".split(";").each { |part| part.split(",").each { |x| parts << x.upcase } }
puts parts.inspect
"ab".chars.each_with_index { |ch, i| print ch * (i + 1), " " }
puts

# each_char as a statement inside a value block (a closure).
per = %w[abc bb].map do |w|
  hits = 0
  w.each_char { |ch| hits += 1 unless ch == "b" }
  hits
end
puts per.inspect

# == on String? both ways, &. chains, and narrowing an index before using it.
none = "abc"[9]
one = "abc"[0]
puts (none == "a").inspect, (one == "a").inspect, (none == nil).inspect, "abc"[9]&.upcase&.downcase.inspect, "abc"[1]&.upcase&.downcase.inspect
hello = "hello"
at = hello.index("l")
puts hello[at].inspect if at
puts (hello.index("z") || -1), (hello.index("h") || -1)

# String == against String? values from [] and Hash#[], and on strings built in a block.
puts ("a" == "abc"[0]).inspect, ("a" == "abc"[9]).inspect, (:a == [:a][0]).inspect
kv = { "k" => "v" } #: Hash[String, String]
puts ("v" == kv["k"]).inspect, (kv["k"] == "v").inspect, (kv["zz"] == "v").inspect
built = ["a", "b"].map { |ch| ch + "" }
puts built.include?("a").inspect, (built[0] == "a").inspect, (built == ["a", "b"]).inspect
