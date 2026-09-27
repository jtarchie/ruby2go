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
