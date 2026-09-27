# rbs_inline: enabled

# adding a key during each raises, like MRI
add_h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
begin
  add_h.each do |k, v|
    puts "visit #{k}"
    add_h["new#{k}"] = v
  end
rescue => e
  puts e.class, e.message
end
puts add_h.inspect

# Data and Struct keys match by value
Point = Data.define(:x, :y) #: [Integer, Integer]
data_h = {} #: Hash[Point, String]
data_h[Point.new(x: 1, y: 2)] = "p"
puts data_h[Point.new(x: 1, y: 2)].inspect
data_h[Point.new(x: 1, y: 2)] = "q"
puts data_h.size
Pair = Struct.new(:a, :b) #: [String, Integer]
data_s = {} #: Hash[Pair, Integer]
data_s[Pair.new("k", 1)] = 1
puts data_s.key?(Pair.new("k", 1)).inspect, [Pair.new("k", 1), Pair.new("k", 1)].tally.size

# each_pair with one block param yields the pair
pair_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
pair_h.each_pair { |x| puts x.inspect }

# instances of an empty class key by identity
class Token
end

tok_a = Token.new
tok_b = Token.new
puts tok_a.equal?(tok_b).inspect, (tok_a == tok_b).inspect
seen = {} #: Hash[Token, Integer]
seen[tok_a] = 1
seen[tok_b] = 2
puts seen.size, seen[tok_a].inspect
puts ({ "t" => tok_a } == { "t" => tok_b }).inspect

# == with optional values ignores order
opt_a = { "a" => 1, "b" => nil } #: Hash[String, Integer?]
opt_b = { "b" => nil, "a" => 1 } #: Hash[String, Integer?]
puts (opt_a == opt_b).inspect
opt_c = { "s" => "x" } #: Hash[String, String?]
opt_d = { "s" => "x" } #: Hash[String, String?]
puts (opt_c == opt_d).inspect
u1 = { 1 => "x", "k" => nil }
u2 = { "k" => nil, 1 => "x" }
puts (u1 == u2).inspect

# == against untyped literals; MRI's 1 == 1.0 is true
lit_e = {} #: Hash[String, Integer]
puts (lit_e == {}).inspect
lit_t = { "a" => 1 } #: Hash[String, Integer]
lit_u = {}
lit_u["a"] = 1
puts (lit_t == lit_u).inspect, (lit_u == lit_t).inspect
lit_s = { a: 1 } #: Hash[Symbol, Integer]
puts (lit_s == { a: 1, b: "x" }.reject { |k, _v| k == :b }).inspect
puts ({ 1 => 1 } == { 1 => 1.0 }).inspect

# an explicit nil fetch default is a value, not "no default"
fetch_h = { "a" => 1 } #: Hash[String, Integer]
puts fetch_h.fetch("a", nil).inspect
puts fetch_h.fetch("x", nil).inspect
cfg = { debug: false } #: Hash[Symbol, untyped]
puts cfg.fetch(:nope, nil).inspect

# a negative count to first is MRI's ArgumentError
first_h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
begin
  puts first_h.first(-1).inspect
rescue ArgumentError => e
  puts e.message
end

# hash literals are not frozen
frozen_h = { "a" => 1 } #: Hash[String, Integer]
puts frozen_h.frozen?.inspect
puts({}.frozen?.inspect)

# Hash.new with no default
new_h = Hash.new #: Hash[String, Integer]
new_h["a"] = 1
puts new_h.inspect, new_h.size

# symbol keys print as labels only when MRI's would
emo = { :"😀" => 2, :"a😀" => 3, :"😀?" => 23, :"٣" => 25 } #: Hash[Symbol, Integer]
puts emo.inspect
mix = { é: 1, "@iv": 1, "$g": 1, "a=": 1, A: 1, a?: 1, "+": 1, "[]": 1, "1a": 1, "a b": 1, "`": 1 } #: Hash[Symbol, Integer]
puts mix.inspect

# != compares by content, ignoring order
ne_a = { "x" => 1, "y" => 2 } #: Hash[String, Integer]
ne_b = { "y" => 2, "x" => 1 } #: Hash[String, Integer]
ne_c = { "x" => 1 } #: Hash[String, Integer]
puts (ne_a != ne_b).inspect, (ne_a != ne_c).inspect, (ne_a != ne_a).inspect

# optional keys, and group_by/tally keyed by optional values
nk = { nil => 1, "a" => 2 } #: Hash[String?, Integer]
puts nk[nil].inspect, nk["a"].inspect, nk.key?("a").inspect
nk_s = "b"
nk[nk_s] = 3
puts nk["b"].inspect, nk.size
nk["b"] = 4
puts nk.size
optv_h = { "a" => 1, "b" => 1, "c" => nil, "d" => nil } #: Hash[String, Integer?]
puts optv_h.group_by { |_k, v| v }.size
puts optv_h.values.tally.size

# self-referencing containers inspect as {...}/[...]
rec_h = {} #: Hash[String, untyped]
rec_h["me"] = rec_h
rec_h["n"] = 1
puts rec_h.size
puts rec_h.inspect

rec_a = [1] #: Array[untyped]
rec_a << rec_a
puts rec_a.inspect
rec_g = {} #: Hash[Symbol, untyped]
rec_g[:a] = [rec_g, rec_a]
puts rec_g.inspect
puts [rec_a, rec_a].inspect

# select/filter/reject with one block param yield the pair
sel_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
sel_h.select { |x| puts x.inspect; true }
sel_h.filter { |x| puts x.inspect; false }
sel_h.reject { |x| puts x.inspect; false }

# _ block params
us_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
us_h.each { |_, v| puts v }
puts us_h.map { |_, v| v * 3 }.inspect
puts us_h.select { |k, _| k.size > 1 }.inspect
us_n = 0
us_h.each_key { |_| us_n += 1 }
puts us_n

# sorting on untyped values and keys reaches rbCmp
scores = { "b" => 2, "a" => 1, "c" => 3 } #: Hash[String, untyped]
puts scores.max_by { |_k, v| v }.inspect
puts scores.min_by { |_k, v| v }.inspect
puts scores.sort_by { |_k, v| v }.inspect
byk = { 2 => "b", 1 => "a" } #: Hash[untyped, String]
puts byk.sort.inspect, byk.min.inspect
people = [{ name: "a", age: 30 }, { name: "b", age: 20 }] #: Array[Hash[Symbol, untyped]]
puts people.min_by { |p| p.fetch(:age) }.inspect

# unused block params
ub_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
puts ub_h.select { |k, v| v > 1 }.inspect
puts ub_h.map { |k, v| "#{k}=#{v}" }.inspect
puts ub_h.find { |k, v| v > 1 }.inspect
puts ub_h.min_by { |k, v| k.size }.inspect

# when Hash matches without the value being used

#: (untyped) -> String
def kind(x)
  case x
  when Hash then "hash"
  else "other"
  end
end

puts kind({ "a" => 1 }), kind(1)
