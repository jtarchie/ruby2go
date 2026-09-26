# skip: a hash literal where Hash[K, V]? is expected ignores the expectation (genHash only looks through TClass), so fetch(k, {}), `{}` into a ?Hash[...]? param or return, a `#: Hash[...]?` local, and `a: 1` into Hash[Symbol, untyped]? fail go build

# rbs_inline: enabled

#: (?Hash[String, Integer]?) -> Integer
def count_or(o = nil) = o ? o.size : -1

#: (?Hash[Symbol, untyped]?) -> String
def opts_or(o = nil) = o ? o.inspect : "none"

#: (bool) -> Hash[String, Integer]?
def maybe(b) = b ? {} : nil

n = { "a" => { "b" => 1 } } #: Hash[String, Hash[String, Integer]]
puts n.fetch("x", {}).inspect, n.fetch("a", {}).inspect
puts count_or({}), count_or({ "a" => 1 }), count_or
puts opts_or(a: 1), opts_or
puts maybe(true).inspect, maybe(false).inspect
m = {} #: Hash[String, Integer]?
puts m.inspect
