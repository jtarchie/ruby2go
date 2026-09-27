# rbs_inline: enabled

h = {} #: Hash[String, untyped]
h["me"] = h
h["n"] = 1
puts h.size
puts h.inspect

a = [1] #: Array[untyped]
a << a
puts a.inspect
g = {} #: Hash[Symbol, untyped]
g[:a] = [g, a]
puts g.inspect
puts [a, a].inspect
