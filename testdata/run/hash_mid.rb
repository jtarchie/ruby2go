# rbs_inline: enabled

# Array and Hash keys match by value, not identity
ak = {} #: Hash[Array[Integer], String]
ak[[1, 2]] = "one-two"
puts ak[[1, 2]].inspect, ak.key?([1, 2]).inspect
ak[[1, 2]] = "again"
puts ak.size, ak.inspect
puts [[1], [1], [2]].tally.inspect
puts [1, 2, 3, 4].group_by { |gn| [gn % 2] }.inspect
hk = {} #: Hash[Hash[String, Integer], Integer]
hk[{ "a" => 1 }] = 1
puts hk[{ "a" => 1 }].inspect, hk.key?({ "a" => 1 }).inspect
ek_a = { "a" => 1 } #: Hash[String, Integer]
ek_b = { "a" => 1 } #: Hash[String, Integer]
puts [ek_a, ek_b].uniq.size, [ek_a, ek_b].tally.size, [ek_a, ek_b].group_by { |x| x }.size

# delete/clear inside each follows MRI's iteration order
del_h = { "a" => 1, "b" => 2, "c" => 3, "d" => 4 } #: Hash[String, Integer]
del_h.each do |k, v|
  puts "visit #{k}"
  del_h.delete(k) if v.even?
end
puts del_h.inspect
g = { "a" => 1, "b" => 2, "c" => 3, "d" => 4 } #: Hash[String, Integer]
g.each do |k, v|
  puts "g #{k} #{v}"
  g.delete("c") if k == "a"
end
puts g.inspect
c = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
c.each_value do |v|
  puts "c #{v}"
  c.clear
end
puts c.inspect

# {} literals flow into optional Hash parameters, fetch defaults and results

#: (?Hash[String, Integer]?) -> Integer
def count_or(o = nil) = o ? o.size : -1

#: (?Hash[Symbol, untyped]?) -> String
def opts_or(o = nil) = o ? o.inspect : "none"

#: (bool) -> Hash[String, Integer]?
def maybe(b) = b ? {} : nil

nest = { "a" => { "b" => 1 } } #: Hash[String, Hash[String, Integer]]
puts nest.fetch("x", {}).inspect, nest.fetch("a", {}).inspect
puts count_or({}), count_or({ "a" => 1 }), count_or
puts opts_or(a: 1), opts_or
puts maybe(true).inspect, maybe(false).inspect
m = {} #: Hash[String, Integer]?
puts m.inspect

# a Hash narrowed from untyped is the same object, so writes show through

#: (untyped) -> void
def add_key(x)
  if x.is_a?(Hash)
    x["new"] = 1
  end
end

#: (untyped) -> Integer
def grow(x)
  case x
  when Hash
    x["grown"] = 2
    x.size
  else
    -1
  end
end

nar_h = { "a" => 1 } #: Hash[String, Integer]
add_key(nar_h)
puts nar_h.inspect
puts grow(nar_h), nar_h.size

# inspect of Hash values that may be nil, including struct-class and Array values
class Foo
  #: () -> String
  def inspect = "#<Foo>"
end

ins_inferred = { "a" => nil, "b" => 2 }
puts ins_inferred.inspect
opt = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
puts opt.inspect, opt.to_s
nil_key = { nil => 1, "a" => 2 } #: Hash[String?, Integer]
puts nil_key.inspect
objs = { "f" => Foo.new } #: Hash[String, Foo?]
puts objs.inspect
lists = { "l" => [1] } #: Hash[String, Array[Integer]?]
puts lists.inspect

# a stored nil and a missing key both read as nil
look_h = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
look_v = look_h["a"]
puts look_v.nil?.inspect
puts look_h["a"].nil?.inspect, look_h["zz"].nil?.inspect, look_h["b"].nil?.inspect
if look_h["a"]
  puts "truthy"
else
  puts "falsy"
end
puts((look_h["a"] || 7).to_s)
look_inferred = { "x" => nil, "y" => 1 }
puts look_inferred["x"].nil?.inspect
d = look_h.delete("a")
puts d.nil?.inspect, look_h.size
puts look_h["a"].inspect
look_h["c"] = nil
puts look_h.delete("c").inspect

# collections built from untyped lookups keep working as values
people = [{ name: "a", age: 30 }, { name: "b", age: 20 }]
names = people.map { |p| p[:name] }
puts names.size
puts names.join(",")
puts (names == ["a", "b"]).inspect
puts names.include?("a").inspect
cfg = { port: 8080, host: "h" } #: Hash[Symbol, untyped]
pair = [cfg[:port], cfg[:host]]
puts pair.size
puts pair.inspect
puts names.inspect
picked = { "p" => cfg[:port], "h" => cfg[:host] }
puts picked.size
puts picked.inspect

# typed Hashes pass to untyped-valued parameters, and an untyped {} to a typed one

#: (Hash[Symbol, untyped]) -> Integer
def opts_size(h) = h.size

#: (Hash[untyped, untyped]) -> Integer
def any_size(h) = h.size

#: (Hash[String, Integer]) -> Integer
def total(h) = h.values.reduce(0) { |a, b| a + b }

ti = { a: 1 } #: Hash[Symbol, Integer]
puts opts_size(ti)
typed = { "n" => 1 } #: Hash[String, Integer]
puts any_size(typed)
loose = {}
loose["a"] = 2
puts total(loose)

groups = {} #: Hash[String, Array[Integer]]
[["a", 1], ["b", 2], ["a", 3]].each { |k, v| (groups[k] ||= []) << v }
p groups
memo = {} #: Hash[Integer, String]
p(memo[1] ||= "one", memo[1] ||= "uno", memo)
arr = [1, 2, 3]
arr[0] += 10
arr[-1] *= 2
p arr, (arr[1] -= 1)
counts = {"x" => 0}
counts["x"] += 5
p counts
nested = {} #: Hash[Symbol, Hash[Symbol, Integer]]
(nested[:a] ||= {})[:b] = 1
p nested
flags = {} #: Hash[String, bool]
flags["on"] ||= true
p flags

hs_h = {a: 1, b: 2, c: 3}
p hs_h.transform_values { |v| v * 10 }, hs_h.transform_keys(&:to_s), hs_h.to_h { |k, v| [v, k] }, hs_h.to_h, hs_h.invert, hs_h.key(2), hs_h.key(9), hs_h.value?(3), hs_h.has_value?(0), hs_h.member?(:a)
p hs_h.values_at(:a, :z), hs_h.fetch_values(:a, :b), hs_h.slice(:a, :c, :z), hs_h.except(:b), hs_h.store(:d, 4), hs_h
p hs_h.merge({a: 100, e: 5}) { |k, old, new| old + new }, hs_h.count { |k, v| v.odd? }, hs_h.any? { |k, v| v > 3 }, hs_h.all? { |k, v| v > 0 }, hs_h.none? { |k, v| v > 9 }
hs_h2 = hs_h.merge({})
hs_h2.delete_if { |k, v| v.even? }
p hs_h2, hs_h.reject { |k, v| v > 2 }, hs_h.select { |k, v| v > 2 }
hs_h.update({z: 26})
hs_h.keep_if { |k, v| v != 1 }
p hs_h, hs_h.sort_by { |k, v| -v }.first(2), hs_h.min_by { |k, v| v }, hs_h.sum { |k, v| v }, hs_h.map { |k, v| "#{k}=#{v}" }.join("&")
p hs_h.find { |k, v| v > 3 }, hs_h.sort, hs_h.max_by { |k, v| v }, hs_h.to_a.last, hs_h.each_with_index.map { |(k, v), i| "#{i}:#{k}" }
