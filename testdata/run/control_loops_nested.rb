# rbs_inline: enabled

arr = [3, 8, 0, 5] #: Array[Integer]
i = 0
while i < arr.size && arr[i] != 0
  i += 1
end
puts i

opt = [1, 2] #: Array[Integer]
j = 0
until opt[j].nil?
  j += 1
end
puts j

# break from an if/else arm leaves only the inner iterator
found = [] #: Array[String]
[1, 2, 3].each do |a|
  [10, 20, 30].each do |b|
    if b > 10 * a
      break
    else
      found << "#{a}-#{b}"
    end
  end
end
puts found.inspect

k = 0
odds = [] #: Array[Integer]
while k < 6
  k += 1
  if k.even?
    if k == 4
      next
    end
    odds << -k
    next
  end
  odds << k
end
puts odds.inspect

w = 0
while true
  break
end
puts w

n = 0
total = 0
(total += n; n += 1) while n < 4
puts total

res = [] #: Array[String]
%w[a b c].each_with_index do |s, idx|
  2.times do |t|
    next if t == idx
    res << "#{s}#{t}"
  end
end
puts res.inspect

#: (Hash[String, Integer], Integer) -> String?
def key_for(h, v)
  h.each do |k, val|
    return k if val == v
  end
  nil
end
hh = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
puts key_for(hh, 2).inspect, key_for(hh, 3).inspect

#: (Array[Integer]) -> Integer
def count_until_neg(xs)
  count = 0
  xs.each do |x|
    break if x < 0
    count += 1
  end
  count
end
puts count_until_neg([1, 2, -1, 4]), count_until_neg([])

acc = [] #: Array[Integer]
10.downto(1) do |d|
  next if d.odd?
  break if d < 4
  acc << d
end
puts acc.inspect

[1, 2, 3, 4].select(&:even?).each do |e|
  puts "even #{e}"
  break
end

"abcd".each_char do |ch|
  next if ch == "b"
  print ch
end
puts

# break and next inside a while stay in the while when it sits in a closure block
sums = [3, 5].map do |lim|
  sum = 0
  m = 0
  while true
    m += 1
    next if m == 2
    break if m > lim
    sum += m
  end
  sum
end
puts sums.inspect
