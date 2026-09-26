# rbs_inline: enabled

#: (Integer) -> Integer
def first_square_over(n)
  1.upto(100) do |i|
    return i if i * i > n
  end
  -1
end

#: (Integer, Integer) -> Array[Integer]
def countdown(from, to)
  out = [] #: Array[Integer]
  from.downto(to) { |i| out << i }
  out
end

#: (Integer) -> Integer
def triangle(n)
  sum = 0
  n.times { |i| sum += i + 1 }
  sum
end

#: (Integer) { (Integer) -> void } -> void
def each_even(n)
  n.times { |i| yield i if i.even? }
end

#: (Integer) { (Integer) -> void } -> void
def rep(n, &blk)
  n.times(&blk)
end

#: (Integer, Integer) { (Integer, Integer) -> void } -> void
def grid_each(w, h)
  h.times { |y| w.times { |x| yield x, y } }
end

class Integer
  #: () { (Integer) -> void } -> void
  def each_digit
    s = to_s
    s.size.times { |i| yield s[i].to_i }
  end
end

seen = [] #: Array[Integer]
3.times { |i| seen << i }
puts seen.inspect

puts "-- zero and negative counts run nothing"
seen = []
0.times { |i| seen << i }
-2.times { |i| seen << i }
5.upto(4) { |i| seen << i }
0.downto(1) { |i| seen << i }
puts seen.inspect

puts "-- next and break"
seen = []
10.times do |i|
  next if i.odd?
  break if i > 6
  seen << i
end
puts seen.inspect
seen = []
1.upto(10) do |i|
  break if i == 4
  seen << i
end
10.downto(1) do |i|
  next unless i % 4 == 0
  seen << i
end
puts seen.inspect

puts "-- upto and downto bounds are inclusive"
seen = []
1.upto(4) { |i| seen << i * i }
-2.upto(-1) { |i| seen << i }
3.upto(3) { |i| seen << i }
puts seen.inspect
puts countdown(3, -1).inspect, countdown(0, 1).inspect, countdown(2, 2).inspect

puts "-- return from inside an iterator"
puts first_square_over(50).inspect, first_square_over(0).inspect, first_square_over(99_999).inspect
puts triangle(0).inspect, triangle(1).inspect, triangle(100).inspect

puts "-- nesting"
total = 0
1.upto(3) do |i|
  i.downto(1) do |j|
    next if j == 2
    total += i * j
  end
end
puts total.inspect
grid = [] #: Array[String]
2.times { |r| 3.times { |c| grid << "#{r}#{c}" } }
puts grid.inspect
acc = 0.0
4.times { |i| acc += i.to_f / 2.0 }
puts acc.inspect

puts "-- yield from inside an iterator, forwarded blocks, reopened Integer"
each_even(7) { |i| print i, " " }
puts
rep(2) { |i| puts "rep #{i}" }
grid_each(2, 2) { |x, y| print "#{x},#{y} " }
puts
digits = [] #: Array[Integer]
1203.each_digit { |d| digits << d }
puts digits.inspect

puts "-- iterators inside value blocks, with computed bounds"
r = [1, 2, 3].map do |k|
  s = 0
  10.times do |i|
    break if i > k
    next if i.zero?
    s += i
  end
  s
end
puts r.inspect
n = 3
n.downto(n - 5) { |i| print i, " " }
puts
2.upto(3.succ) { |i| print i }
puts
