# rbs_inline: enabled

# A named block passed on to each/each_pair re-yields (decision 29).
#: (Hash[String, Integer]) { ([String, Integer]) -> void } -> void
def walk(h, &blk)
  h.each(&blk)
end

#: (Hash[String, Integer]) { (String, Integer) -> void } -> void
def walk_pairs(h, &blk)
  h.each_pair(&blk)
end

#: (Hash[String, Integer]) { (String, Integer) -> void } -> void
def walk_yield(h)
  h.each { |k, v| yield k, v }
end

# return from inside each still runs ensure.
#: (Hash[String, Integer]) -> String
def find_big(h)
  h.each { |k, v| return k if v > 1 }
  "none"
ensure
  puts "ensure"
end

#: (Hash[String, Integer], Integer) -> String?
def first_over(h, limit)
  h.each { |k, v| return k if v > limit }
  nil
end

#: (Hash[String, Integer]) -> Integer
def sum_until_negative(h)
  total = 0
  h.each_value do |v|
    break if v < 0
    total += v
  end
  total
end

#: (Hash[String, Integer]) -> Array[String]
def keys_via_each_key(h)
  out = [] #: Array[String]
  h.each_key { |k| out << k }
  out
end

h = { "a" => 1, "b" => 5, "c" => -2, "d" => 7 } #: Hash[String, Integer]
h.each { |k, v| puts "each #{k}=#{v}" }
h.each { |pair| puts pair.inspect }
h.each_pair { |k, v| puts "pair #{k}=#{v}" }
h.each_key { |k| print k, " " }
puts
h.each_value { |v| print v, " " }
puts

# each/each_value are iterators (decision 4): return and break are plain Go.
puts first_over(h, 4).inspect, first_over(h, 100).inspect, first_over({}, 0).inspect
puts sum_until_negative(h), sum_until_negative({})
puts keys_via_each_key(h).inspect

h.each_pair do |k, v|
  next if v.odd?
  puts "even #{k}"
end
h.each_key do |k|
  next unless k > "b"
  puts "late #{k}"
end

found = nil #: String?
h.each do |k, v|
  if v == 7
    found = k
    break
  end
end
puts found.inspect

acc = 0
h.each_with_index do |pair, i|
  acc += i * 10 + pair[1]
  break if pair[1] < 0
end
puts acc
h.each_with_index { |pair, i| puts "#{i}:#{pair[0]}" }
h.each_entry { |pair| print pair[0] }
puts

# Pairs are [K, V] tuples: index, first/last, destructuring.
h.each do |pair|
  k, v = pair
  puts "#{pair.first}#{pair.last} #{k}#{v}"
end
puts h.map(&:first).inspect, h.map(&:last).inspect

e = {} #: Hash[String, Integer]
e.each { |k, _v| puts "never #{k}" }
e.each_key { |k| puts "never #{k}" }
e.each_value { |v| puts "never #{v}" }
e.each_pair { |k, _v| puts "never #{k}" }

# Iteration follows insertion order after deletes and re-inserts.
ord = {} #: Hash[Integer, String]
[5, 3, 9, 1].each { |n| ord[n] = n.to_s }
ord.delete(3)
ord[3] = "three"
ord[5] = "five"
ord.each { |k, v| print k, v, " " }
puts

# 20k keys in a scattered order, then two-thirds deleted: order must survive.
big = {} #: Hash[Integer, Integer]
i = 0
while i < 20_000
  big[(i * 7919) % 20_000] = i
  i += 1
end
puts big.size, big.keys.first(5).inspect, big.values.first(5).inspect
j = 0
while j < 20_000
  big.delete(j) if j % 3 != 0
  j += 1
end
puts big.size, big.first(4).inspect
total = 0
big.each_key { |k| total += k }
puts total
last = nil #: Integer?
big.each_key { |k| last = k }
puts last.inspect

w = { "a" => 1, "bb" => 2, "ccc" => 3 } #: Hash[String, Integer]
walk(w) { |k, v| puts "#{k}:#{v}" }
walk(w) { |pair| puts pair.inspect }
walk_pairs(w) { |k, v| puts "#{k}-#{v}" }
walk_yield(w) { |k, v| puts "#{k}+#{v}" }
puts find_big(w), find_big({})

# Numbered and `it` block params: _1 alone and `it` are the pair.
puts w.map { _1 }.inspect, w.map { "#{_1}=#{_2}" }.inspect
puts w.map { it }.inspect, w.sort_by { -it[1] }.inspect
w.each { puts it.inspect }

# Exceptions leave the loop; a rescue inside the body keeps it going.
begin
  w.each { |k, v| raise ArgumentError, k if v > 1 }
rescue ArgumentError => e
  puts "rescued #{e.message}"
end
w.each do |k, v|
  begin
    raise "x#{k}" if v.odd?
    puts "ok #{k}"
  rescue => e
    puts e.message
  end
end

# Overwriting existing keys during iteration is allowed and keeps order.
w.each { |k, v| w[k] = v * 10 }
puts w.inspect
w.each_key { |k| w[k] = (w[k] || 0) + 1 }
puts w.inspect
# Deleting while walking a keys snapshot is the safe pattern.
w.keys.each { |k| w.delete(k) if k != "bb" }
puts w.inspect
w.clear
w.each { |k, _v| puts "never #{k}" }
w["z"] = 1
w.each_pair { |k, v| puts "#{k}#{v}" }
snap = w.map { |k, v| w[k] = v + 1; k }
puts snap.inspect, w.inspect
