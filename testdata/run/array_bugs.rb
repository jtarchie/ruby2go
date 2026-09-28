# rbs_inline: enabled

# a block on a Hash#[] result (Array?) receiver
hopt = { "a" => [1, 2] } #: Hash[String, Array[Integer]]
puts hopt["a"].map { |x| x * 2 }.inspect
hopt["a"].each { |x| puts x }

# unused block params still bind positionally
bp_nums = [1, 2] #: Array[Integer]
bp_nums.each { |n| puts n }
bp_nums.each_with_index { |n, i| puts i }
pairs = [[1, "a"]] #: Array[[Integer, String]]
pairs.each { |k, s| puts k, s }
puts pairs.map { |k, s| k }.inspect

# compact drops nils from Array[Integer?]
cmp_opt = [1, nil, 3, nil] #: Array[Integer?]
puts cmp_opt.compact.size
cmp_opt.compact.each { |x| puts x.inspect }

# == across element types
ints = [1, 2] #: Array[Integer]
floats = [1.0, 2.0] #: Array[Float]
eq_u = [1, 2] #: Array[untyped]
puts ints == floats, ints == eq_u, eq_u == ints
nested = [[1, 2], [], [3]] #: Array[Array[Integer]]
puts nested == [[1, 2], [], [3]]

# join flattens nested arrays
join_a = [] #: Array[untyped]
join_a << 3
join_a << [1, [2]]
puts join_a.join(",")
join_grid = [[1, 2], [3]] #: Array[Array[Integer]]
puts join_grid.join("-")

# map with a void block collects nils
void_nums = [1, 2] #: Array[Integer]
puts void_nums.map { |n| puts n }.inspect

# != compares by value, not identity
ne_a = [1, 2] #: Array[Integer]
ne_b = [1, 2] #: Array[Integer]
puts ne_a != ne_b, ne_a != ne_a, ne_a != [3], [1] != [1]

# include?(nil) on Array[Integer?]
inc_opt = [1, nil, 3] #: Array[Integer?]
puts inc_opt.include?(nil).inspect
puts inc_opt.include?(3).inspect

# nil elements inspect as nil
insp_opt = [1, nil, 3] #: Array[Integer?]
puts insp_opt.inspect
puts insp_opt.to_s
puts "interp #{insp_opt}"
tail = [1, nil] #: Array[Integer?]
puts tail.last.inspect
puts tail.pop.inspect
lit = [4, nil]
puts lit.inspect

# join and puts with nil elements
jp_opt = [1, nil, 3] #: Array[Integer?]
puts jp_opt.join("-").inspect
puts jp_opt

# indexing Array[Integer?] yields a flat Integer?
idx_opt = [1, nil, 3] #: Array[Integer?]
puts idx_opt[1].nil?, idx_opt[0].nil?, idx_opt[9].nil?
if (hole = idx_opt[1])
  puts "truthy #{hole.inspect}"
else
  puts "falsy"
end
puts idx_opt[1].inspect

# puts of empty and nested-empty arrays
empty = [] #: Array[Integer]
puts empty
puts [[], [1]]
puts "end"

# sort/max_by with an optional (Hash#[]) key
counts = { "a" => 3, "b" => 7, "c" => 5 } #: Hash[String, Integer]
key_words = ["a", "b", "c"] #: Array[String]
puts key_words.max_by { |w| counts[w] }.inspect
puts key_words.min_by { |w| counts[w] }.inspect
puts key_words.sort_by { |w| [counts[w], w] }.inspect
sort_opt = [3, 1, 2] #: Array[Integer?]
puts sort_opt.sort.inspect, sort_opt.max.inspect

# a splat rest param is a fresh array, not the caller's

#: (*String) -> Array[String]
def collect(*parts) = parts

alias_words = ["p", "q"] #: Array[String]
got = collect(*alias_words)
got[0] = "z"
puts alias_words.inspect, got.inspect

# splatting typed and untyped arrays into a rest param

#: (*Integer) -> Integer
def total_elem(*ns) = ns.reduce(0) { |acc, n| acc + n }

elem_nums = [1, 2, 3] #: Array[Integer]
puts(*elem_nums)
elem_u = [] #: Array[untyped]
elem_u << 4
elem_u << 5
puts total_elem(*elem_u)

# splats mixed with plain args

#: (*Integer) -> Integer
def total_mixed(*ns) = ns.reduce(0) { |acc, n| acc + n }

mixed_nums = [1, 2, 3] #: Array[Integer]
more = [4] #: Array[Integer]
puts total_mixed(10, *mixed_nums)
puts total_mixed(*mixed_nums, 5)
puts total_mixed(*mixed_nums, *more)

# tally/group_by key nested arrays by value
tally_grid = [[1], [1], [2]] #: Array[Array[Integer]]
puts tally_grid.tally.inspect
puts tally_grid.group_by { |r| r }.size

# to_a returns self, so << aliases
copy_a = [1, 2] #: Array[Integer]
copy_b = copy_a.to_a
copy_b << 3
puts copy_a.inspect, copy_a.to_a.equal?(copy_a)

# a tuple of optionals inspects nil

#: (Array[Integer]) -> [Integer?, Integer?]
def bounds(nums) = [nums.min, nums.max]

puts bounds([]).inspect
puts bounds([2]).inspect

# Array[Integer] passes where Array[untyped] is expected

#: (Array[untyped]) -> String
def show(xs) = xs.map { |x| x.inspect }.join(" ")

show_nums = [1, 2] #: Array[Integer]
puts show(show_nums)
puts ([] + show_nums).inspect

# uniq matches arrays, Structs and Data by value
Pair = Struct.new(:a, :b) #: [Integer, Integer]
Val = Data.define(:v) #: [Integer]
uniq_grid = [[1], [1], [2]] #: Array[Array[Integer]]
puts uniq_grid.uniq.inspect
ps = [Pair.new(1, 2), Pair.new(1, 2)] #: Array[Pair]
puts ps.uniq.size
vs = [Val.new(1), Val.new(1), Val.new(2)] #: Array[Val]
puts vs.uniq.size

# sort/min/max on an unannotated [] of integers
loose = []
loose << 3
loose << 1
loose << 2
puts loose.sort.inspect, loose.min.inspect, loose.max.inspect, loose.sort_by { |x| x }.inspect

# sort_by/max_by with an array key; sort of nested arrays
key_nums = [3, 1, 2] #: Array[Integer]
puts key_nums.sort_by { |n| [n % 2, n] }.inspect
puts key_nums.max_by { |n| [n, -n] }.inspect
key_grid = [[2, 1], [1, 5]] #: Array[Array[Integer]]
puts key_grid.sort.inspect, key_grid.min.inspect

# a top-level generic def

#: [T] (Array[T]) -> T?
def second(arr) = arr[1]

puts second([1, 2, 3]).inspect, second(["a"]).inspect

# T? in a predicate block is truthiness; bare Array.new takes its annotation
tb_h = { "a" => 1, "b" => nil } #: Hash[String, Integer?]
puts tb_h.select { |_k, v| v }.inspect
tb_h2 = { 1 => 2 } #: Hash[Integer, Integer]
tb_nums = [1, 2] #: Array[Integer]
puts tb_nums.select { |n| tb_h2[n] }.inspect
puts ["a", "b"].select { |s| s =~ /a/ }.inspect
tb_a = Array.new #: Array[Integer]
tb_a << 1
puts tb_a.inspect
