# rbs_inline: enabled

#: (Integer) -> String
def size_word(n)
  if n > 100
    word = "large"
  elsif n > 10
    word = "medium"
  else
    word = "small"
  end
  word + "!"
end
puts size_word(1000), size_word(50), size_word(0)

#: (Symbol) -> Integer
def weight(s)
  case s
  when :a
    w = 1
  when :b
    w = 2
  else
    w = 0
  end
  w * 10
end
puts weight(:a), weight(:b), weight(:c)

#: (Integer) -> String
def parse(d)
  begin
    step = "start"
    q = 100 / d
    step = "divided"
  rescue ZeroDivisionError
    puts "rescue sees #{step}"
    q = -1
  ensure
    puts "ensure sees #{step}"
  end
  "#{step} #{q}"
end
puts parse(5), parse(0)

#: (Integer) -> String
def nil_first(n)
  if n > 0
    label = nil
  else
    label = "non-positive"
  end
  label.inspect
end
puts nil_first(1), nil_first(0)

i = 0
while i < 3
  last = i * 10
  i += 1
end
puts last

unless i.zero?
  msg = "i=#{i}"
else
  msg = "zero"
end
puts msg

maybe = nil
maybe = "now" if i == 3
puts maybe.inspect
unset = nil
unset = "never" if i == 99
puts unset.inspect

count = nil #: Integer?
puts count.inspect
count = 5
puts count.inspect
count = nil
puts count.inspect
count = -1
puts count.inspect

sum = 0
doubled = [1, 2, 3].map { |x| sum += x; x * 2 }
puts sum, doubled.inspect

seen = 0
[4, 5].each { |v| seen = v }
puts seen

grid = [] #: Array[String]
[1, 2].each do |r|
  [3, 4].each do |c|
    cell = "#{r}x#{c}"
    grid << cell
  end
end
puts grid.inspect

#: (Array[Integer]) -> Array[Integer]
def running(xs)
  total = 0
  xs.map { |x| total += x }
end
puts running([1, 2, 3, -6]).inspect, running([]).inspect

#: (Integer) -> Integer
def bump(n)
  n += 1
  n *= 2
  n -= 3
  n
end
puts bump(4), bump(-4), bump(0)

s = "a"
s += "b"
s *= 2
puts s.inspect
f = 1.5
f /= 2
f -= 0.25
puts f.inspect
b = 7
b %= 4
b **= 3
b /= 2
puts b.inspect
nb = -7
nb /= 2
puts nb.inspect
nm = -7
nm %= 3
puts nm.inspect
list = [1] #: Array[Integer]
list += [2, 3]
puts list.inspect

x1 = 1
x1 = x1 + 1 while x1 < 100
puts x1

#: (Integer) -> String
def shadow(v)
  out = [] #: Array[String]
  [v, v + 1].each do |w|
    tmp = "#{w}!"
    out << tmp
  end
  tmp = "outer"
  out << tmp
  out.join(",")
end
puts shadow(1)

nums = [1, 2, 3] #: Array[Integer]
evens = [] #: Array[Integer]
odds = 0
nums.each do |n|
  if n.even?
    evens << n
  else
    odds += 1
  end
end
puts evens.inspect, odds

flag = false
nums.each { |n| flag = true if n > 2 }
puts flag

it_sum = 0
nums.each { it_sum += it }
puts it_sum
