# skip: blocked on container-eq-instantiation: Array[untyped] == Array[String] is false across Go instantiations (the rest passes since untyped? collapses to untyped)

# rbs_inline: enabled

people = [{ name: "a", age: 30 }, { name: "b", age: 20 }]
names = people.map { |p| p[:name] }
puts names.size
puts names.join(",")
puts (names == ["a", "b"]).inspect
puts names.include?("a").inspect
cfg = { port: 8080, host: "h" } #: Hash[Symbol, untyped]
pair = [cfg[:port], cfg[:host]]
puts pair.size
puts pair.inspect
puts names.inspect
picked = { "p" => cfg[:port], "h" => cfg[:host] }
puts picked.size
puts picked.inspect
