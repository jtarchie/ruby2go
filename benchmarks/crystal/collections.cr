# Crystal port of benchmarks/collections.rb, for the README comparison column.
# Struct is a `record`; pairs are Tuples, so Ruby's `[[k, v], ...]` inspect output is written by hand.

# sort_by, sort with a block, group_by, tally, partition, uniq, min_by/max_by, Set, Struct, Symbol keys, flat_map.
require "set"

record Person, name : String, age : Int32, city : Symbol

def pairs_inspect(pairs) : String
  "[" + pairs.join(", ") { |(k, v)| "[#{k.inspect}, #{v.inspect}]" } + "]"
end

cities = %i[paris tokyo lima oslo cairo]
people = Array.new(100_000) { |i| Person.new("p#{i * 7919 % 100_003}", 18 + (i * 31 % 60), cities[i % 5]) }

puts people.sort_by { |p| {p.age, p.name} }.first(3).map(&.name).inspect
puts people.sort { |a, b| b.age <=> a.age }.first.age
by_city = people.group_by(&.city)
puts pairs_inspect(by_city.transform_values(&.size).to_a.sort)
age, count = people.map(&.age).tally.max_by { |_, c| c }
puts "[#{age}, #{count}]"
young, old = people.partition { |p| p.age < 40 }
puts young.size, old.size
puts people.min_by(&.age).name, people.max_by(&.name).name
puts people.map { |p| p.age // 10 }.uniq.sort.inspect

seen = Set(String).new
dups = 0
people.each { |p| dups += 1 unless seen.add?(p.name[0, 4]) }
puts seen.size, dups

h = {} of Symbol => Int32
people.each { |p| h[p.city] = (h[p.city]? || 0) + p.age }
puts pairs_inspect(h.to_a.sort_by { |(k, v)| {-v, k} })
puts people.flat_map { |p| [p.age, p.age * 2] }.sum
puts people.each_slice(1000).map { |s| s.sum(&.age) }.max
puts people.select { |p| p.city == :lima }.reject { |p| p.age.odd? }.size
puts (1..200_000).to_a.reverse.sort.bsearch { |x| x >= 123_456 }
