# rbs_inline: enabled

#: (Array[Integer]) -> void
def grow(list)
  list << list.size
end

PRIMES = [2, 3, 5, 7] #: Array[Integer]

#: (Integer) -> bool
def prime?(n) = PRIMES.include?(n)

#: (?Array[Integer]) -> Integer
def count_all(xs = []) = xs.size

#: (Array[Integer]) -> Array[Integer]
def evens(nums)
  return [] if nums.empty?
  nums.select(&:even?)
end

#: (bool) -> Array[String]
def pick(b) = b ? ["x"] : []

nums = [10, 20, 30] #: Array[Integer]
puts nums[0].inspect, nums[2].inspect, nums[3].inspect, nums[100].inspect
puts nums[-1].inspect, nums[-3].inspect, nums[-4].inspect
puts nums.last.inspect, nums.size.inspect, nums.length.inspect, nums.empty?.inspect

empty = [] #: Array[Integer]
puts empty[0].inspect, empty[-1].inspect, empty.last.inspect, empty.size.inspect, empty.empty?.inspect

nums[1] = 21
nums[-1] = 31
puts nums.inspect
nums[3] = 40
puts nums.inspect, nums.size
puts (nums[0] = 11).inspect, nums.inspect

puts nums.fetch(0).inspect, nums.fetch(-1).inspect, nums.fetch(-4).inspect
begin
  nums.fetch(4)
rescue IndexError => e
  puts e.message, e.class
end
begin
  nums.fetch(-5)
rescue IndexError => e
  puts e.message
end
begin
  empty.fetch(0)
rescue IndexError => e
  puts e.message
end
begin
  nums[-9] = 1
rescue IndexError => e
  puts e.class
end

puts nums.push(50).inspect, (nums << 60).inspect, nums.append(70).inspect
puts nums.pop.inspect, nums.pop.inspect, nums.inspect
puts nums.shift.inspect, nums.inspect
puts nums.unshift(5).inspect
puts empty.pop.inspect, empty.shift.inspect, empty.inspect
puts nums.concat([1, 2]).inspect, nums.concat([]).inspect
puts (nums + [3]).inspect, nums.inspect, (empty + empty).inspect
puts nums.reverse.inspect, nums.inspect, empty.reverse.inspect

chain = [1] #: Array[Integer]
chain << 2 << 3
puts chain.inspect
puts chain.push(4).equal?(chain), chain.append(5).equal?(chain), chain.unshift(0).equal?(chain)
puts chain.concat([6]).equal?(chain), (chain + []).equal?(chain), chain.reverse.equal?(chain)

copy = nums.dup
copy << 99
copy[0] = -1
puts copy.inspect, nums.inspect, copy.equal?(nums)
alias_of = nums
alias_of << 77
puts nums.inspect, alias_of.equal?(nums)
grow(nums)
grow(empty)
puts nums.inspect, empty.inspect

puts [3, 1, 3, 2, 1, 2].uniq.inspect, ["b", "a", "b"].uniq.inspect, empty.uniq.inspect
puts [4, 5].compact.inspect
puts nums.delete(21).inspect, nums.delete(1000).inspect, nums.inspect
dup_vals = [1, 2, 1, 3, 1] #: Array[Integer]
puts dup_vals.delete(1).inspect, dup_vals.inspect
words = ["x", "y", "x"] #: Array[String]
puts words.delete("x").inspect, words.inspect
puts nums.delete_at(0).inspect, nums.delete_at(-1).inspect, nums.delete_at(100).inspect, nums.delete_at(-100).inspect, nums.inspect
puts nums.clear.inspect, nums.empty?.inspect, nums.size, nums.clear.equal?(nums)

queue = [1] #: Array[Integer]
seen = [] #: Array[Integer]
until queue.empty?
  x = queue.shift
  next unless x
  seen << x
  queue << x * 2 << x * 2 + 1 if x < 4
end
puts seen.inspect

stack = [] #: Array[String]
"a(b(c)d)e".each_char do |ch|
  if ch == "("
    stack.push(ch)
  elsif ch == ")"
    stack.pop
  end
end
puts stack.empty?

total = 0
until seen.empty?
  top = seen.pop
  total += top if top
end
puts total, seen.inspect

big = [9223372036854775807, -9223372036854775808, 0] #: Array[Integer]
puts big.inspect, big[0].inspect, big.fetch(1).inspect

grid = [[1], [2]] #: Array[Array[Integer]]
gcopy = grid.dup
gcopy[0]&.push(9)
gcopy << [3]
puts grid.inspect, gcopy.inspect, gcopy.equal?(grid)
row = grid[0]
row << 7 if row
puts grid.inspect, grid[1]&.size.inspect, grid[5]&.size.inspect
gap = []
gap[2] = 1
puts gap.inspect, gap.size
ogap = [1] #: Array[Integer?]
ogap[3] = 4
puts ogap.size

puts big.is_a?(Array), big.is_a?(Enumerable), big.is_a?(Object), big.is_a?(Integer), big.kind_of?(Array)
puts big.respond_to?(:each), big.respond_to?(:nope), big.nil?
puts prime?(5), prime?(4), PRIMES.size
PRIMES << 11
puts prime?(11), PRIMES.last.inspect

puts count_all, count_all([1, 2]), evens([]).inspect, evens([1, 2, 4]).inspect, pick(true).inspect, pick(false).inspect
out = [] #: Array[Integer]
out = evens([6]) if out.empty?
puts out.inspect

puts "before"
nums.fetch(3)
puts "not reached"
