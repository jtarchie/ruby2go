# rbs_inline: enabled

# yield, closures, procs and lambdas, inject/each_with_object, each_slice/each_cons and Enumerator.

#: (Integer n) { (Integer) -> Integer } -> Integer
def fold_up(n)
  acc = 0
  i = 0
  while i < n
    acc += yield(i)
    i += 1
  end
  acc
end

#: (Integer k) -> ^(Integer) -> Integer
def adder(k) = ->(x) { x + k }

total = 0
20.times { |j| total += fold_up(20_000) { |i| (i ^ j) & 255 } }
puts total

add3 = adder(3)
twice = proc { |x| x * 2 } #: ^(Integer) -> Integer
composed = add3 >> twice
puts (1..300_000).sum { |i| composed.call(i) % 11 }

counter = 0
bump = -> { counter += 1 }
200_000.times { bump.call }
puts counter

nums = (1..300_000).to_a
puts nums.inject(0) { |acc, n| acc + (n % 7) }
puts nums.each_with_object(Hash.new(0)) { |n, h| h[n % 13] += 1 }.values.max
puts nums.each_slice(4).sum { |s| s.first }
puts nums.each_cons(3).count { |w| w.sum % 5 == 0 }
puts nums.each_with_index.sum { |n, i| n * (i % 3) } % 1_000_003
puts nums.lazy.map { |n| n * 3 }.select(&:even?).first(5).inspect
puts nums.zip(nums.reverse).sum { |a, b| a * (b || 0) % 17 }
