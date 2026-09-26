# rbs_inline: enabled

i = 0
while i < 3
  i += 1
end
puts i.inspect

j = 10
until j <= 7
  j -= 1
end
puts j.inspect

never = 0
while false
  never += 1
end
until true
  never += 1
end
puts never

k = 0
k += 2 while k < 9
puts k
m = 5
m -= 1 until m.zero?
puts m
neg = -3
neg += 1 while neg.negative?
puts neg

acc = [] #: Array[Integer]
n = 0
while true
  n += 1
  next if n.odd?
  break if n > 8
  acc << n
end
puts acc.inspect, n

# break and next only affect the innermost loop
out = [] #: Array[String]
a = 0
while a < 3
  b = 0
  while true
    b += 1
    break if b > a
    next if b == 2
    out << "#{a}#{b}"
  end
  a += 1
end
puts out.inspect

# the condition is re-evaluated each time round
xs = [3, 1, 2] #: Array[Integer]
popped = [] #: Array[Integer]
until xs.empty?
  popped << xs.size
  xs.pop
end
puts popped.inspect, xs.inspect

#: (Integer) -> Integer
def collatz(n)
  steps = 0
  while n != 1
    n = n.even? ? n / 2 : 3 * n + 1
    steps += 1
  end
  steps
end
puts collatz(1), collatz(6), collatz(27)

total = 0
10.times do |t|
  next if t.even?
  break if t > 7
  total += t
end
puts total
0.times { puts "never" }
neg.pred.pred.times { puts "never" }

steps = [] #: Array[Integer]
5.downto(1) do |d|
  next if d == 3
  steps << d
end
1.upto(0) { |u| steps << 100 + u }
-1.upto(1) { |u| steps << u }
puts steps.inspect

words = ["a", "b", "c", "d"] #: Array[String]
words.each_with_index do |s, idx|
  next if idx.zero?
  break if s == "d"
  puts "#{idx}:#{s}"
end
words.reverse_each do |s|
  next if s == "c"
  puts s
  break if s == "b"
end
h = { "x" => 1, "y" => 2, "z" => 3 } #: Hash[String, Integer]
h.each do |key, val|
  next if val == 1
  puts "#{key}=#{val}"
  break if key == "y"
end

#: (Array[String]) -> Integer?
def index_of_empty(list)
  list.each_with_index do |s, idx|
    return idx if s.empty?
  end
  nil
end
puts index_of_empty(["a", "", "b"]).inspect, index_of_empty(["a"]).inspect,
  index_of_empty([]).inspect

#: (Array[Array[Integer]], Integer) -> String
def find_pair(rows, target)
  rows.each_with_index do |row, r|
    row.each_with_index do |v, c|
      return "#{r},#{c}" if v == target
    end
  end
  "missing"
end
grid = [[1, 2], [3, 4]] #: Array[Array[Integer]]
puts find_pair(grid, 4), find_pair(grid, 1), find_pair(grid, 9)

#: (Integer) -> Integer?
def first_square_over(limit)
  i = 1
  while true
    return i * i if i * i > limit
    return nil if i > 100
    i += 1
  end
end
puts first_square_over(50).inspect, first_square_over(0).inspect,
  first_square_over(100_000).inspect

#: (Integer) -> void
def void_early(n)
  return if n > 1
  puts "small #{n}"
end
void_early(1)
void_early(5)

# a user iterator with ensure: break, next and raise all run the ensure

#: () { (Integer) -> void } -> void
def with_cleanup
  yield 1
  yield 2
ensure
  puts "cleanup"
end

with_cleanup do |v|
  puts "got #{v}"
  break if v == 1
end
with_cleanup do |v|
  next if v == 1
  puts "next skipped to #{v}"
end
begin
  with_cleanup { |v| raise ArgumentError, "from block #{v}" }
rescue ArgumentError => e
  puts "outside: #{e.message}"
end

#: (Array[Integer]) -> Integer?
def find_neg(list)
  with_cleanup do |v|
    return v * 100 if list.include?(-v)
  end
  nil
end
puts find_neg([-2]).inspect, find_neg([]).inspect

#: (Integer) { (Integer) -> void } -> void
def countdown(from)
  while from > 0
    yield from
    from -= 1
  end
end
countdown(5) do |c|
  next if c == 4
  break if c == 2
  puts "countdown #{c}"
end

# a method that rescues around yield takes a closure; next is a return

#: () { (Integer) -> void } -> void
def guarded
  [1, 2, 3].each { |g| yield g }
rescue => e
  puts "guarded caught #{e.message}"
end

guarded do |g|
  next if g == 2
  puts "guarded #{g}"
  raise "stop at #{g}" if g == 3
end

kinds = [] #: Array[String]
v = 0
while v < 5
  v += 1
  case v
  when 2, 4 then next
  when 5 then kinds << "last"
  else kinds << "v#{v}"
  end
end
puts kinds.inspect

#: (Array[untyped]) -> String
def first_string(items)
  items.each do |item|
    case item
    when String then return "string #{item}"
    when nil then next
    end
  end
  "no string"
end
puts first_string([1, nil, "s", "t"]), first_string([nil]), first_string([])
