# rbs_inline: enabled

# User code can reopen Hash; K and V are in scope.
class Hash
  #: () -> Integer
  def double_size = size * 2

  #: () -> K?
  def second_key = keys[1]
end

h = { "b" => 2, "a" => 1, "c" => 3 } #: Hash[String, Integer]
puts h.inspect
puts h.keys.inspect, h.values.inspect

# Reassigning an existing key keeps its position; a new key goes last.
h["a"] = 10
h["d"] = 4
puts h.inspect

# delete returns the value, or nil for a miss; a re-added key goes last.
puts h.delete("b").inspect, h.delete("b").inspect, h.delete("zz").inspect
puts h.inspect
h["b"] = 20
puts h.inspect

puts h["a"].inspect, h["nope"].inspect, h[""].inspect
puts h.size.inspect, h.length.inspect, h.empty?.inspect
puts h.key?("a").inspect, h.key?("A").inspect
puts h.has_key?("b").inspect, h.include?("x").inspect, h.include?("d").inspect

puts h.fetch("c").inspect, h.fetch("zz", -1).inspect, h.fetch("c", 99).inspect
puts h.fetch("zz", 0).inspect

# []= returns the assigned value.
r = (h["e"] = 5)
puts r.inspect, h.size

e = {} #: Hash[String, Integer]
puts e.inspect, e.size.inspect, e.empty?.inspect, e.keys.inspect, e.values.inspect
puts e["x"].inspect, e.delete("x").inspect, e.key?("").inspect

# clear returns self and empties in place; the hash stays usable.
c = h.clear
puts c.inspect, c.equal?(h).inspect, h.size, h.empty?.inspect
h["z"] = 26
h["y"] = 25
puts h.inspect, h.keys.inspect

# keys/values are copies.
ks = h.keys
ks << "extra"
vs = h.values
vs << 0
puts ks.inspect, h.keys.inspect, h.values.inspect

# Assignment aliases the same hash.
alias_h = h
alias_h["x"] = 24
puts h.inspect, h.size

# merge keeps the receiver's order, appends new keys, and changes neither operand.
m1 = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
m2 = { "b" => 20, "c" => 30 } #: Hash[String, Integer]
puts m1.merge(m2).inspect, m2.merge(m1).inspect
puts m1.inspect, m2.inspect
puts m1.merge({}).inspect, e.merge(m1).inspect, e.merge(e).inspect
puts({ "q" => 0 }.merge(m1).inspect)

ints = { 3 => "c", -1 => "neg", 0 => "zero", 4_611_686_018_427_387_904 => "big" } #: Hash[Integer, String]
puts ints.inspect
puts ints[-1].inspect, ints[0].inspect, ints[4_611_686_018_427_387_904].inspect, ints[1].inspect
ints[-1] = "minus one"
puts ints.keys.inspect, ints[-1].inspect

# Float keys: -0.0 and 0.0 are the same key, as in Ruby.
floats = { 0.5 => 1, -0.0 => 2, 1e20 => 3, 2.0 => 4 } #: Hash[Float, Integer]
puts floats.inspect
puts floats[0.0].inspect, floats[2.0].inspect, floats[0.25].inspect

bools = { true => "t", false => "f" } #: Hash[bool, String]
puts bools[false].inspect, bools[true].inspect, bools.size
sym = { alpha: 1, beta: 2 } #: Hash[Symbol, Integer]
puts sym[:alpha].inspect, sym[:gamma].inspect, sym.key?(:beta).inspect, sym.keys.inspect

uni = { "héllo" => 1, "日本" => 2, "😀" => 3, "" => 4 } #: Hash[String, Integer]
puts uni["日本"].inspect, uni["😀"].inspect, uni[""].inspect, uni["hello"].inspect
puts uni.keys.map { |k| k.size }.inspect

# A String key and a Symbol key with the same text are different keys.
sk = { a: 1, "a" => 2 } #: Hash[untyped, Integer]
puts sk[:a].inspect, sk["a"].inspect, sk.size

# No default-value hash (decision 5), so buckets are created explicitly.
lists = {} #: Hash[String, Array[Integer]]
[3, 1, 4, 1, 5].each do |n|
  key = n.odd? ? "odd" : "even"
  bucket = lists[key]
  if bucket
    bucket << n
  else
    lists[key] = [n]
  end
end
puts lists.inspect

# The decision-5 replacements for Hash.new(0) + `+= 1`.
counts = {} #: Hash[String, Integer]
"the quick the lazy the end quick".split.each { |w| counts[w] = (counts[w] || 0) + 1 }
puts counts.inspect
counts2 = {} #: Hash[String, Integer]
"b a b".split.each { |w| counts2[w] = counts2.fetch(w, 0) + 1 }
puts counts2.inspect

# Nested hashes: the inner hash is shared, not copied.
nested = { "x" => { "y" => 1 }, "e" => {} } #: Hash[String, Hash[String, Integer]]
inner = nested["x"]
if inner
  inner["z"] = 2
end
puts nested.inspect
puts nested["x"]&.fetch("z").inspect, nested["nope"]&.fetch("z").inspect

# Reopened methods work on every instantiation.
puts h.double_size, h.second_key.inspect, e.second_key.inspect, sym.second_key.inspect

# Optional values: a stored nil is still a key.
ov = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
puts ov.size, ov.key?("a").inspect, ov.keys.inspect
ov.each { |k, v| puts "#{k} #{v.nil?} #{v.to_s}" }
ov.each_value { |v| puts v.inspect }
puts ov.fetch("a").inspect, ov.fetch("b").inspect, ov.fetch("a", 9).inspect
ov["a"] = 5
puts ov.select { |_k, v| !v.nil? }.keys.inspect, ov.map { |k, v| v ? k : "-" }.inspect

# Assignment in a condition narrows the lookup.
if (hit = h["z"])
  puts hit + 1
end
puts (h.delete("z") || 0) + 1, (h.delete("z") || 0) + 1

# More bucket idioms without a default value.
all = {} #: Hash[String, Array[Integer]]
[1, 2, 3].each { |n| all["all"] = (all["all"] || []) + [n] }
puts all.inspect
cnt = {} #: Hash[Symbol, Integer]
%i[a b a c a].each { |s| cnt[s] = cnt.fetch(s, 0) + 1 }
puts cnt.inspect, cnt.max_by { |_k, v| v }.inspect

# Value-returning block calls through &. on a possibly-nil nested hash
# (iterators through &. are dynamic_bug_safe_nav_iterator).
puts nested["x"]&.map { |nk, nv| "#{nk}#{nv}" }.inspect, nested["zz"]&.map { |nk, _nv| nk }.inspect
puts nested["x"]&.select { |_nk, nv| nv > 1 }.inspect, nested["x"]&.keys.inspect
