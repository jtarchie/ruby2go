# Crystal port of benchmarks/blocks.rb, for the README comparison column.
# Crystal Procs have no >> composition, so `composed` is a lambda calling both;
# Ruby's lazy enumerator is Crystal's Iterator (lazy by default); Int64 where sums overflow Int32.

# yield, closures, procs and lambdas, inject/each_with_object, each_slice/each_cons and Enumerator.

def fold_up(n : Int32, &) : Int32
  acc = 0
  i = 0
  while i < n
    acc += yield(i)
    i += 1
  end
  acc
end

def adder(k : Int32) : Int32 -> Int32
  ->(x : Int32) { x + k }
end

total = 0
20.times { |j| total += fold_up(20_000) { |i| (i ^ j) & 255 } }
puts total

add3 = adder(3)
twice = ->(x : Int32) { x * 2 }
composed = ->(x : Int32) { twice.call(add3.call(x)) }
puts (1..300_000).sum { |i| composed.call(i) % 11 }

counter = 0
bump = -> { counter += 1 }
200_000.times { bump.call }
puts counter

nums = (1..300_000).to_a
puts nums.reduce(0) { |acc, n| acc + (n % 7) }
puts nums.each_with_object(Hash(Int32, Int32).new(0)) { |n, h| h[n % 13] += 1 }.values.max
puts nums.each_slice(4).sum { |s| s.first.to_i64 }
puts nums.each_cons(3).count { |w| w.sum % 5 == 0 }
puts nums.each_with_index.sum { |(n, i)| n.to_i64 * (i % 3) } % 1_000_003
puts nums.each.map { |n| n * 3 }.select(&.even?).first(5).to_a.inspect
puts nums.zip(nums.reverse).sum { |(a, b)| a.to_i64 * b % 17 }
