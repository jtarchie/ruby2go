# rbs_inline: enabled

#: (Array[Integer], Integer) -> Integer?
def index_of(arr, target)
  arr.each_with_index { |x, i| return i if x == target }
  nil
end

#: (Array[String]) -> String
def first_long(words)
  words.each do |w|
    next if w.size < 4
    return w
  end
  "none"
end

#: (Integer) { (Integer, String) -> void } -> void
def pairs_upto(n)
  i = 0
  while i < n
    yield i, i.to_s * 2
    i += 1
  end
end

#: (Array[Integer]) { (Integer) -> void } -> void
def each_even(nums)
  nums.each { |n| yield n if n.even? }
end

#: (Array[Integer]) { (Integer) -> void } -> void
def safe_each(nums)
  nums.each do |n|
    begin
      yield n
    rescue ZeroDivisionError => e
      puts "rescued #{e.message}"
    end
  end
end

#: (Array[Integer]) { (Integer) -> Integer } -> Integer
def sum_by(nums)
  total = 0
  nums.each { |n| total += yield(n) }
  total
end

#: (Array[Array[Integer]], Integer) -> Integer
def find_cell(grid, target)
  grid.each_with_index do |row, r|
    row.each_with_index do |v, c|
      return r * 10 + c if v == target
    end
  end
  -1
end

#: (Array[String]) { (String) -> bool } -> Integer
def count_where(xs, &blk) = xs.select(&blk).size

#: (Array[Integer]) -> void
def show_all(nums) = nums.each { |n| puts "all #{n}" }

#: (Array[Integer]) -> void
def show_rev(nums)
  nums.reverse_each { |n| puts "rev #{n}" }
end

class Bag
  #: () -> void
  def initialize
    @items = [] #: Array[String]
  end

  #: (String) -> self
  def add(s)
    @items << s
    self
  end

  #: () { (String) -> void } -> void
  def each(&block) = @items.each(&block)

  #: () { (String) -> void } -> void
  def each_twice(&block)
    @items.each(&block)
    @items.reverse_each(&block)
  end

  #: () { (String) -> bool } -> Array[String]
  def keep(&block) = @items.select(&block)

  #: () { (String) -> Integer } -> Integer
  def score(&block)
    acc = 0
    @items.each { |s| acc += block.call(s) }
    acc
  end
end

class Util
  #: [T] (Array[T]) -> T?
  def self.second(arr) = arr[1]

  #: [T, U] (Array[T]) { (T) -> U } -> Array[U]
  def self.my_map(arr)
    out = [] #: Array[U]
    arr.each { |x| out << yield(x) }
    out
  end

  #: [T] (Array[T], T) -> Array[T]
  def self.without(arr, drop) = arr.reject { |x| x == drop }
end

nums = [4, 8, 15, 16, 23, 42] #: Array[Integer]
nums.each { |n| puts n if n.even? }
nums.each do |n|
  next if n < 10
  break if n > 20
  puts "mid #{n}"
end
nums.each_with_index { |n, i| puts "#{i}->#{n}" if i.odd? }
nums.each_with_index do |n, i|
  break if i > 1
  puts "ewi #{n}"
end
nums.reverse_each { |n| print n, " " }
puts
nums.reverse_each do |n|
  next if n.odd?
  break if n < 16
  puts "revbreak #{n}"
end
nums.each_entry { |n| puts "entry #{n}" if n > 30 }
[].each { |x| puts "never #{x}" }
puts index_of(nums, 15).inspect, index_of(nums, 99).inspect, index_of([], 1).inspect
puts first_long(["a", "bb", "cccc", "ddddd"]).inspect, first_long([]).inspect
each_even(nums) { |n| print n, "," }
puts
each_even(nums) do |n|
  break if n > 10
  puts "even #{n}"
end

nums.each { puts _1 * 100 if _1 > 40 }
nums.each { puts it - 1 if it < 5 }
puts nums.map { _1 + 1 }.inspect, nums.select { it.odd? }.inspect
nums.each_with_index { puts "#{_2}:#{_1}" if _2.zero? }

safe_each([1, 0, 2]) { |n| puts 10 / n }
puts sum_by([1, 2, 3]) { |n| n * n }, sum_by([]) { |n| n }

pairs_upto(3) { |i, s| puts "#{i}/#{s}" }
pairs_upto(5) do |i, s|
  next if i == 1
  break if i == 3
  puts "#{i}|#{s}"
end
pairs_upto(2) { |i| puts "only #{i}" }
pairs_upto(0) { |i, s| puts "never #{i}#{s}" }

bag = Bag.new.add("x").add("yy").add("zzz")
bag.each { |s| puts s }
bag.each_twice { |s| print s, " " }
puts
bag.each do |s|
  break if s.size > 1
  puts "bag first #{s}"
end
puts bag.keep { |s| s.size > 1 }.inspect, bag.score { |s| s.size * 10 }
puts Util.second([1, 2, 3]).inspect, Util.second(["a"]).inspect, Util.second([[1], [2, 3]]).inspect
puts Util.my_map([1, 2]) { |n| n.to_s + "!" }.inspect, Util.my_map(["a"]) { |s| s.size }.inspect
puts Util.without([1, 2, 1, 3], 1).inspect, Util.without(["a", "b"], "c").inspect

total = 0
[1, 2, 3].each { |n| total += n }
puts total
acc = [] #: Array[String]
%w[a b c].each_with_index { |s, i| acc << s * (i + 1) }
puts acc.inspect
counter = 0
doubled = [1, 2, 3].map do |n|
  counter += 1
  n * counter
end
puts doubled.inspect, counter
labels = nums.map do |n|
  if n > 20
    "big"
  elsif n.odd?
    "odd"
  else
    "even"
  end
end
puts labels.inspect

grid = [[1, 2], [3, 4], []] #: Array[Array[Integer]]
grid.each { |row| row.each { |c| print c, ";" } }
puts
grid.each_with_index do |row, r|
  row.each_with_index { |c, col| puts "(#{r},#{col})=#{c}" if c.even? }
end
puts grid.map { |row| row.map { |c| c * c } }.inspect, grid.flat_map { |row| row }.inspect
puts grid.map { |row| row.reduce(0) { |a, b| a + b } }.inspect, grid.select(&:empty?).size
puts grid.map(&:size).inspect, grid.sort_by(&:size).map(&:size).inspect
puts [1, 2, 3].then { |a| a.size }.inspect, [3, 1].then { |a| a.sort }.inspect

fns = [] #: Array[Integer]
[1, 2, 3].each do |n|
  captured = n * 10
  fns << captured
end
puts fns.inspect

sq = [[1, 2, 3], [4, 5, 6]] #: Array[Array[Integer]]
puts find_cell(sq, 5), find_cell(sq, 1), find_cell(sq, 9)
sq.each do |row|
  row.each do |v|
    next if v.even?
    break if v > 4
    print v, " "
  end
  print "| "
end
puts
puts sum_by([-1, 2, -3], &:abs)
puts count_where(["a", "", "b"], &:empty?), count_where(["a", "", "b"]) { |s| !s.empty? }
small = [1, 2, 3] #: Array[Integer]
sums = small.map do |n|
  r = 0
  small.each do |m|
    next if m == 2
    break if m > n
    r += m
  end
  r
end
puts sums.inspect
found = small.select do |n|
  hit = false
  small.each_with_index { |m, i| hit = true if m == n && i.odd? }
  hit
end
puts found.inspect
show_all([1, 2])
show_rev([3, 4])
each_even([2, 4, 12, 14]) do |n|
  pairs_upto(3) do |i, s|
    break if i > 1
    next if i.zero?
    puts "nest #{n} #{s}"
  end
  break if n > 10
  puts "after #{n}"
end
