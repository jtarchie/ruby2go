# skip: instances of a class with no ivars are Go zero-size structs, whose pointers compare equal, so distinct objects are equal?/== and collapse into one Hash key; MRI keeps them apart

# rbs_inline: enabled

class Token
end

a = Token.new
b = Token.new
puts a.equal?(b).inspect, (a == b).inspect
seen = {} #: Hash[Token, Integer]
seen[a] = 1
seen[b] = 2
puts seen.size, seen[a].inspect
puts ({ "t" => a } == { "t" => b }).inspect
