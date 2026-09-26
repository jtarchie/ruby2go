# rbs_inline: enabled

h = { "a" => 1, "bb" => 2, "ccc" => 3 } #: Hash[String, Integer]

# Hash#select/filter/reject return a Hash, unlike Enumerable's Array.
puts h.select { |_k, v| v.odd? }.inspect
puts h.filter { |k, _v| k.size > 1 }.inspect
puts h.reject { |_k, v| v == 2 }.inspect
puts h.select { |_k, _v| false }.inspect, h.reject { |_k, _v| false }.inspect
sel = h.select { |_k, v| v > 0 }
sel["new"] = 9
puts h.size, sel.size

puts h.map { |k, v| "#{k}=#{v}" }.inspect
puts h.map { |pair| pair }.inspect
puts h.map { |k, v| [k.upcase, v * 10] }.inspect
puts h.flat_map { |k, v| [k, v.to_s] }.inspect
puts h.find { |_k, v| v > 1 }.inspect, h.detect { |_k, v| v > 5 }.inspect
puts h.any? { |_k, v| v > 2 }.inspect, h.any? { |_k, v| v > 3 }.inspect
puts h.all? { |_k, v| v > 0 }.inspect, h.all? { |k, _v| k.size < 3 }.inspect
puts h.none? { |k, _v| k.empty? }.inspect, h.none? { |k, _v| k == "a" }.inspect
puts h.count
puts h.reduce(0) { |sum, pair| sum + pair[1] }
puts h.inject("") { |acc, pair| acc + pair[0] }
puts h.to_a.inspect
puts h.include?("a").inspect, h.include?("z").inspect

# Pairs compare element-wise, so sort/min/max order by key.
puts h.sort.inspect, h.min.inspect, h.max.inspect
puts h.sort_by { |_k, v| -v }.inspect
puts h.min_by { |_k, v| v }.inspect, h.max_by { |k, _v| k.size }.inspect
puts h.group_by { |_k, v| v.odd? }.inspect
puts h.tally.inspect
puts h.first(2).inspect, h.take(1).inspect, h.first(0).inspect, h.first(10).inspect

ints = { 3 => "c", -1 => "neg", 0 => "zero" } #: Hash[Integer, String]
puts ints.sort.inspect, ints.min.inspect, ints.max.inspect
puts ints.min_by { |_k, v| v }.inspect, ints.sort_by { |_k, v| v.size }.inspect

sym = { b: 2, a: 1, c: 0 } #: Hash[Symbol, Integer]
puts sym.sort.inspect, sym.sort_by { |_k, v| v }.inspect, sym.min_by { |k, _v| k }.inspect

st = { "Zed" => "b", "alpha" => "a", "Élan" => "c", "beta" => "d" } #: Hash[String, String]
puts st.sort.inspect
puts st.sort_by { |k, _v| k.downcase }.map { |k, _v| k }.inspect

fl = { "x" => 2.5, "y" => -1.5 } #: Hash[String, Float]
puts fl.sort_by { |_k, v| v }.inspect, fl.max_by { |_k, v| v }.inspect

arrv = { "b" => [2, 1], "a" => [1] } #: Hash[String, Array[Integer]]
puts arrv.sort.inspect, arrv.max_by { |_k, v| v.size }.inspect

e = {} #: Hash[String, Integer]
puts e.min.inspect, e.max.inspect, e.min_by { |_k, v| v }.inspect, e.sort.inspect, e.to_a.inspect
puts e.find { |_k, _v| true }.inspect, e.any? { |_k, _v| true }.inspect, e.all? { |_k, _v| false }.inspect
puts e.reduce(10) { |s, _pair| s + 1 }, e.count, e.first(3).inspect
puts e.group_by { |k, _v| k }.inspect, e.tally.inspect, e.flat_map { |k, _v| [k] }.inspect
puts e.select { |_k, _v| true }.inspect, e.map { |k, _v| k }.inspect

# Word counting as in example 05, sorted by count then word.
words = "the cat and the hat and the bat".split
counts = words.tally
puts counts.inspect
puts counts.sort_by { |w, n| [-n, w] }.first(3).inspect
puts words.group_by { |w| w.size }.inspect
puts ["é", "e", "é"].tally.inspect, [].tally.inspect

# Chains: Hash#select returns a Hash, so Hash methods keep working after it.
puts h.select { |_k, v| v > 1 }.map { |k, v| "#{k}#{v}" }.inspect
puts h.reject { |_k, v| v > 1 }.keys.inspect
puts h.merge({ "d" => 4 }).select { |k, _v| k.size < 3 }.size
puts h.select { |_k, v| v > 1 }.reject { |k, _v| k.size > 2 }.map { |k, v| k * v }.sort.first(1).inspect
puts h.sort_by { |_k, v| -v }.first(2).map { |k, _v| k }.inspect
puts h.to_a.sort.reverse.inspect
puts h.map { |k, v| [v, k] }.sort.inspect, h.map { |k, v| [v, k] }.max.inspect
puts h.keys.map(&:upcase).inspect, h.values.map { |v| v * 2 }.inspect
puts h.map { |k, v| { k => v } }.inspect
h.map { |k, v| [k, v] }.each { |k, v| print k, v }
puts
puts h.values.sort.reverse.inspect, h.keys.max.inspect, h.keys.min_by { |k| -k.size }.inspect
puts h.inject(0) { |s, pair| s + pair.last }
pair = h.find { |_k, v| v == 2 }
if pair
  k, v = pair
  puts k, v
end
puts h.min_by { |_k, v| v }&.first.inspect, h.max_by { |_k, v| v }&.last.inspect

# Symbol procs on pairs.
puts h.sort_by(&:last).inspect, h.min_by(&:last).inspect, h.max_by(&:first).inspect
puts h.group_by(&:last).inspect, h.map(&:inspect).inspect

# Kernel and Object methods on a Hash.
puts h.then { |x| x.size }, h.class, h.class.inspect
puts h.is_a?(Hash).inspect, h.is_a?(Enumerable).inspect, h.nil?.inspect
puts h.respond_to?(:each).inspect, h.respond_to?(:nope).inspect
puts h.select { |_k, v| v > 5 }.empty?.inspect, (!h).inspect

# Other key types sort by their own <=>.
ss = { a: :z, b: :y, c: :x } #: Hash[Symbol, Symbol]
puts ss.sort_by { |_k, v| v }.inspect, ss.max_by { |_k, v| v }.inspect, ss.min.inspect
ff = { 2.5 => "x", -1.0 => "y", 0.0 => "z" } #: Hash[Float, String]
puts ff.sort.inspect, ff.max.inspect, ff.min_by { |_k, v| v }.inspect
tt = { [2, "b"] => 1, [1, "z"] => 2, [2, "a"] => 3 } #: Hash[[Integer, String], Integer]
puts tt.sort.inspect, tt.min.inspect, tt.keys.sort.inspect, tt.sort_by { |k, _v| k }.map(&:last).inspect

# Building hashes with reduce, each_with_index and group_by.
fruit = %w[pear apple fig apple pear pear]
init = {} #: Hash[String, Integer]
fc = fruit.reduce(init) do |acc, w|
  acc[w] = acc.fetch(w, 0) + 1
  acc
end
puts fc.inspect, init.equal?(fc).inspect
first_at = {} #: Hash[String, Integer]
fruit.each_with_index { |w, i| first_at[w] = i unless first_at.key?(w) }
puts first_at.inspect
by_size = fruit.group_by { |w| w.size }
puts by_size.map { |n, ws| "#{n}:#{ws.uniq.join("/")}" }.inspect, by_size.keys.sort.inspect, by_size.fetch(4).size
puts fc.group_by { |w, _n| w.size }.inspect
puts fc.sort_by { |w, n| [-n, w] }.map { |w, n| "#{w}=#{n}" }.join(" ")
inv = {} #: Hash[Integer, Array[String]]
fc.each do |w, n|
  list = inv[n]
  if list
    list << w
  else
    inv[n] = [w]
  end
end
puts inv.inspect
