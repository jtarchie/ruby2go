# rbs_inline: enabled

require "set"

plain = Set.new([3, 1, 2])
puts plain.sort.inspect

from_range = Set.new(1..4) #: Set[Integer]
puts from_range.sort.inspect

from_set = Set.new(from_range) #: Set[Integer]
puts from_set.sort.inspect

h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
from_hash = Set.new(h) #: Set[[String, Integer]]
puts from_hash.sort_by { |pair| pair[0] }.inspect

doubled = Set.new([1, 2, 3]) { |x| x * 2 } #: Set[Integer]
puts doubled.sort.inspect

strs = Set.new(1..3) { |x| x.to_s } #: Set[String]
puts strs.sort.inspect

m = Set.new([1, 2, 3])
m.map! { |x| x * 10 }
puts m.sort.inspect

sel = Set.new([1, 2, 3, 4, 5, 6])
puts sel.select! { |x| x.even? }.sort.inspect
puts sel.select! { |x| x.even? }.inspect

rej = Set.new([1, 2, 3, 4, 5, 6])
puts rej.reject! { |x| x.even? }.sort.inspect
puts rej.reject! { |x| x.even? }.inspect

grouped = Set.new([1, 2, 3, 4, 5, 6]).classify { |x| x % 3 }
puts grouped.keys.sort.inspect
puts grouped[0].sort.inspect
puts grouped[1].sort.inspect
puts grouped[2].sort.inspect

divided = Set.new([1, 2, 3, 4, 5, 6]).divide { |x| x % 3 }
puts divided.map(&:sort).sort_by(&:first).inspect
