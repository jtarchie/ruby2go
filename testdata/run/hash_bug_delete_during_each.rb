# skip: Hash#delete/clear shrink the key slice in place under a running each: after a delete the next key is skipped and the last key is yielded twice, and after clear the stale keys are yielded with zero values

# rbs_inline: enabled

h = { "a" => 1, "b" => 2, "c" => 3, "d" => 4 } #: Hash[String, Integer]
h.each do |k, v|
  puts "visit #{k}"
  h.delete(k) if v.even?
end
puts h.inspect

# Deleting a key that has not been visited yet: MRI never yields it.
g = { "a" => 1, "b" => 2, "c" => 3, "d" => 4 } #: Hash[String, Integer]
g.each do |k, v|
  puts "g #{k} #{v}"
  g.delete("c") if k == "a"
end
puts g.inspect

# clear during iteration ends it.
c = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
c.each_value do |v|
  puts "c #{v}"
  c.clear
end
puts c.inspect
