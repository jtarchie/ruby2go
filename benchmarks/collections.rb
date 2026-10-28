# rbs_inline: enabled

# sort_by, sort with a block, group_by, tally, partition, uniq, min_by/max_by, Set, Struct, Symbol keys, flat_map.
require "set"

Person = Struct.new(:name, :age, :city) #: [String, Integer, Symbol]

cities = %i[paris tokyo lima oslo cairo]
people = Array.new(100_000) { |i| Person.new("p#{i * 7919 % 100_003}", 18 + (i * 31 % 60), cities[i % 5] || :none) }

puts people.sort_by { |p| [p.age, p.name] }.first(3).map(&:name).inspect
puts people.min { |a, b| b.age <=> a.age }.age
by_city = people.group_by(&:city)
puts by_city.transform_values(&:size).sort.inspect
puts people.map(&:age).tally.max_by { |_, c| c }.inspect
young, old = people.partition { |p| p.age < 40 }
puts young.size, old.size
puts people.min_by(&:age)&.name, people.max_by(&:name)&.name
puts people.map { |p| p.age / 10 }.uniq.sort.inspect

seen = Set.new #: Set[String]
dups = 0
people.each { |p| seen.add?(p.name[0, 4] || "") ? nil : dups += 1 }
puts seen.size, dups

h = {} #: Hash[Symbol, Integer]
people.each { |p| h[p.city] = (h[p.city] || 0) + p.age }
puts h.sort_by { |k, v| [-v, k] }.inspect
puts people.flat_map { |p| [p.age, p.age * 2] }.sum
puts people.each_slice(1000).map { |s| s.sum(&:age) }.max
puts people.select { |p| p.city == :lima }.reject { |p| p.age.odd? }.size
puts((1..200_000).to_a.reverse.sort.bsearch { |x| x >= 123_456 })
