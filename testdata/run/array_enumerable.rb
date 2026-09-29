# rbs_inline: enabled

words = ["pear", "fig", "apple", "fig", "kiwi", "Äpfel", "banana"] #: Array[String]
nums = [5, -3, 0, 12, -3, 7] #: Array[Integer]
none = [] #: Array[Integer]
floats = [2.5, -1.0, 10.0, 0.1] #: Array[Float]

puts nums.map { |n| n * 2 }.inspect, none.map { |n| n * 2 }.inspect
puts words.map { |w| w.size }.inspect, words.map(&:upcase).inspect, nums.map(&:to_s).inspect
puts nums.map { |n| n > 0 }.inspect, nums.map { |n| n.to_f / 2 }.inspect
puts nums.select { |n| n > 0 }.inspect, nums.reject { |n| n > 0 }.inspect, nums.select(&:zero?).inspect
puts none.select { |n| n > 0 }.inspect, none.reject { |n| n > 0 }.inspect, nums.reject(&:negative?).inspect
puts nums.find { |n| n > 6 }.inspect, nums.find { |n| n > 100 }.inspect, nums.detect(&:negative?).inspect, none.detect { |n| n > 0 }.inspect
puts nums.any? { |n| n > 10 }.inspect, nums.any? { |n| n > 100 }.inspect, none.any? { |n| n > 0 }.inspect
puts nums.all? { |n| n > -5 }.inspect, nums.all? { |n| n > 0 }.inspect, none.all? { |n| n > 0 }.inspect
puts nums.none? { |n| n > 100 }.inspect, nums.none?(&:zero?).inspect, none.none? { |n| n > 0 }.inspect
puts nums.count.inspect, none.count.inspect, words.count.inspect
puts nums.reduce(0) { |acc, n| acc + n }.inspect, none.reduce(42) { |acc, n| acc + n }.inspect
puts nums.inject("") { |acc, n| acc + n.to_s }.inspect, words.inject(0) { |acc, w| acc + w.size }.inspect
puts nums.reduce(1) { |acc, n| n.zero? ? acc : acc * n }.inspect, floats.reduce(0.0) { |acc, f| acc + f }.inspect
puts nums.include?(12).inspect, nums.include?(13).inspect, none.include?(0).inspect, words.include?("fig").inspect, words.include?("Fig").inspect
puts floats.include?(0.1).inspect, [0.1 + 0.2].include?(0.3).inspect
puts nums.to_a.inspect, none.to_a.inspect
puts nums.tally.inspect, words.tally.inspect, none.tally.inspect, [true, false, true].tally.inspect
puts nums.first(2).inspect, nums.first(0).inspect, nums.first(100).inspect, none.first(1).inspect
puts nums.take(3).inspect, words.take(1).inspect
nums.each_entry { |n| print n, " " }
puts

puts nums.sort.inspect, words.sort.inspect, none.sort.inspect, floats.sort.inspect, nums.inspect
puts ["b", "B", "a", "é", "Z", "aa", ""].sort.inspect, [:b, :a, :c].sort.inspect
puts words.sort_by { |w| [w.size, w] }.inspect, nums.sort_by { |n| -n }.inspect, none.sort_by { |n| n }.inspect
puts floats.sort_by { |f| -f }.inspect, words.sort_by(&:downcase).inspect
puts nums.min.inspect, nums.max.inspect, none.min.inspect, none.max.inspect
puts words.min.inspect, words.max.inspect, floats.min.inspect, floats.max.inspect
puts words.min_by { |w| w.size }.inspect, words.max_by { |w| w.size }.inspect, none.min_by { |n| n }.inspect, none.max_by { |n| n }.inspect
puts nums.min_by { |n| n.abs }.inspect, nums.max_by(&:abs).inspect
puts words.group_by { |w| w.size }.inspect, nums.group_by { |n| n <=> 0 }.inspect, none.group_by(&:even?).inspect
puts words.flat_map { |w| [w, w.upcase] }.inspect, nums.flat_map { |n| [n] }.inspect, none.flat_map { |n| [n, n] }.inspect
puts nums.flat_map { |n| n > 0 ? [n, n] : none }.inspect

puts nums.select(&:positive?).map { |n| n * n }.reduce(0) { |a, b| a + b }
puts words.map(&:downcase).uniq.sort.first(3).inspect
puts nums.sort.reverse.take(2).inspect, words.reject { |w| w.size > 4 }.map(&:size).tally.inspect
puts words.group_by(&:size).map { |k, v| "#{k}:#{v.size}" }.inspect
puts nums.inspect, words.inspect

acc0 = [] #: Array[Integer]
puts nums.reduce(acc0) { |acc, n| acc + [n * 2] }.inspect, acc0.inspect
puts nums.inject(acc0) { |acc, n| acc << n }.inspect, acc0.inspect
h0 = {} #: Hash[Integer, String]
puts nums.reduce(h0) { |h, n| h[n] = n.to_s * 2; h }.inspect
puts ["bb", "aa", "c"].max_by(&:size).inspect, ["x", "yy", "z"].min_by(&:size).inspect, ["b", "a"].max_by { |_s| 0 }.inspect
puts [0.0, -0.0, 1.0].uniq.inspect, [0.0, -0.0].tally.inspect, [3, 1, 3].max.inspect

odd = [4, 9, 2] #: Array[Integer]
grid = [[1, 2], []] #: Array[Array[Integer]]
puts odd.max&.succ.inspect, none.max&.succ.inspect, (odd.min || 0) + 1, (none.min || 0) + 1
puts words.find { |w| w.size > 5 }&.upcase.inspect, words.find { |w| w.size > 9 }&.upcase.inspect
top = odd.max
puts top * 2 if top
puts (odd[1] || -1) + 1, none[0] || -1, odd.first(1)[0].inspect, odd.sort.last.inspect
puts odd.map { |n| n.to_s }.max_by(&:size).inspect, grid.map { |r| r.max || 0 }.inspect
puts grid.flat_map { |r| r }.sort.reverse.first(2).inspect, odd.reject(&:even?).map { |n| [n, n.to_s] }.inspect
puts odd.sort.map(&:to_s).join("<")

bools = [true, false] #: Array[bool]
syms = [:a, :b] #: Array[Symbol]
flag = false
puts bools.include?(true), bools.include?(false), [true].include?(false), syms.include?(:a), syms.include?(:c)
puts bools.include?(flag), bools.tally.inspect, bools.uniq.inspect, bools.count
puts bools.select { |b| b }.inspect, bools.reject { |b| b }.size, bools.map { |b| !b }.inspect, bools.all? { |b| b }

# grep / grep_v go through pattern === x.
p [1, "a", :b, 2.5, nil].grep(Integer), (1..10).grep(2..4), %w[a bb cb].grep(/b/), [1, 2, 3, 2].grep(2)
grep_mixed = [1, "a", 2.5] #: Array[untyped]
p %w[a bb cb].grep_v(/b/), [:ab, :cd].grep(/a/), grep_mixed.grep_v(Float)
