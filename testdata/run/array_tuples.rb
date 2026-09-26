# rbs_inline: enabled

#: (Integer, Integer) -> [Integer, Integer]
def divmod2(a, b) = [a / b, a % b]

#: (String) -> [String, Integer, bool]
def describe(s) = [s.upcase, s.size, s.empty?]

#: (Array[Integer]) -> [Integer?, Integer?]
def bounds(nums) = [nums.min, nums.max]

#: ([Integer, String]) -> String
def render(pair) = "#{pair[1]}=#{pair[0]}"

#: (untyped) -> String
def show(x) = x.inspect

#: () -> [Array[Integer], Integer]
def split_list = [[1, 2], 3]

pair = [1, "one"]
puts pair.inspect, pair.to_s, pair[0].inspect, pair[1].inspect, pair.first.inspect, pair.last.inspect
puts "interp #{pair}"
trio = [2, "two", :b]
puts trio.inspect, trio[2].inspect, trio.last.inspect, trio.first.inspect
mixed = [1, 2.5]
puts mixed.inspect
nested = [[1, 2], "x"]
puts nested.inspect, nested.first.size, nested[0].inspect
puts (pair == [1, "one"]).inspect, (pair == [2, "one"]).inspect, (pair != [1, "one"]).inspect, (pair != [1, "two"]).inspect
puts ([1, "a"] <=> [1, "b"]).inspect, ([2, "a"] <=> [1, "z"]).inspect, ([1, "a"] <=> [1, "a"]).inspect
puts render([7, "seven"]), render(pair)
puts show([1, "a"]), show([[1, "b"], 2]), show(pair)

q, r = divmod2(17, 5)
puts q, r
q, r = divmod2(-17, 5)
puts q, r
puts divmod2(0, 3).inspect, divmod2(9, -4).inspect
up, len, emp = describe("héllo")
puts up.inspect, len.inspect, emp.inspect
puts describe("").inspect, describe("日本").inspect
lo, hi = bounds([3, 9, -2])
puts lo.inspect, hi.inspect
lo, hi = bounds([])
puts lo.inspect, hi.nil?
x, y = [3, "three"]
puts x.inspect, y.inspect
a = 1
b = 2
a, b = b, a
puts a, b
a, b = b, a + b
puts a, b
first, second = [10, 20, 30] #: Array[Integer]
puts first.inspect, second.inspect
m, n = [5] #: Array[Integer]
puts m.inspect, n.inspect

pairs = [[3, "c"], [1, "a"], [2, "b"]] #: Array[[Integer, String]]
puts pairs.inspect, pairs.size
puts pairs.sort.inspect, pairs.min.inspect, pairs.max.inspect, pairs.reverse.inspect
pairs.each { |num, s| puts "#{num}=#{s}" }
pairs.each { |pr| puts pr.inspect }
pairs.each_with_index { |pr, i| puts "#{i}:#{pr.inspect}:#{pr.last}" }
puts pairs.map { |num, s| s * num }.inspect
puts pairs.select { |num, _s| num.odd? }.inspect, pairs.reject { |num, _s| num.odd? }.inspect
puts pairs.sort_by { |num, s| [-num, s] }.inspect
puts pairs.map { |pr| pr.last }.inspect, pairs.map(&:first).inspect
puts pairs.map { |num, s| [s, num] }.inspect
puts pairs.include?([1, "a"]).inspect, pairs.include?([1, "b"]).inspect
puts pairs.find { |num, _s| num > 1 }.inspect, pairs.any? { |_n, s| s == "b" }.inspect
puts pairs.group_by { |num, _s| num.odd? }.inspect
puts pairs.min_by { |_n, s| s }.inspect, pairs.max_by { |num, _s| num }.inspect
puts pairs.reduce(0) { |acc, pr| acc + pr[0] }.inspect
puts pairs.map { |pr| render(pr) }.inspect
pairs << [0, "z"]
pairs.push([9, "i"])
puts pairs.inspect, pairs.last.inspect, pairs[0].inspect, pairs[99].inspect
dups = [[1, "a"], [1, "a"], [2, "a"]] #: Array[[Integer, String]]
puts dups.uniq.inspect, dups.tally.inspect, dups.delete([1, "a"]).inspect, dups.inspect

puts ["b", "a", "c"].map { |s| [s, s.ord] }.inspect
puts ["bb", "a", "ccc"].map { |s| [s.size, s] }.sort.inspect

ab = [1, 2, 3] #: Array[Integer]
a1, b1 = ab
puts a1.inspect, b1.inspect
w1, w2, w3, w4 = ab
puts w1.inspect, w2.inspect, w3.inspect, w4.inspect
list, cnt = split_list
list << cnt
puts list.inspect, split_list.inspect
cells = {} #: Hash[[Integer, Integer], String]
cells[[0, 1]] = "a"
cells[[2, 3]] = "b"
puts cells[[0, 1]].inspect, cells[[9, 9]].inspect, cells.key?([2, 3]), cells.size
cells[[0, 1]] = "c"
puts cells.inspect
coords = [[0, 1], [0, 1], [1, 1]] #: Array[[Integer, Integer]]
puts coords.uniq.inspect, coords.tally.inspect, coords.include?([1, 1]), coords.sort.inspect, coords.max.inspect
puts coords.map { |cx, cy| cells[[cx, cy]] || "-" }.inspect
trios = [[1, "b", :x], [1, "a", :y], [0, "z", :z]] #: Array[[Integer, String, Symbol]]
puts trios.sort.inspect, trios.min.inspect, trios.max_by { |_n, s, _y| s }.inspect
trios.each { |tn, ts, ty| puts "#{tn}#{ts}#{ty}" }
puts trios.map { |tn, _ts, ty| [ty, tn] }.inspect, trios.last.inspect
pairs.each { puts _1.inspect }
pairs.each { puts "#{_2}#{_1}" }
pairs.each { puts it.inspect }
puts pairs.map { _2 * _1 }.inspect, pairs.map { it.last }.inspect, pairs.select { _1.first > 1 }.inspect
