# skip: a typed Hash passed where Hash[Symbol, untyped] or Hash[untyped, untyped] is expected (or back) is not converted; go build fails

# rbs_inline: enabled

#: (Hash[Symbol, untyped]) -> Integer
def opts_size(h) = h.size

#: (Hash[untyped, untyped]) -> Integer
def any_size(h) = h.size

#: (Hash[String, Integer]) -> Integer
def total(h) = h.values.reduce(0) { |a, b| a + b }

ti = { a: 1 } #: Hash[Symbol, Integer]
puts opts_size(ti)
typed = { "n" => 1 } #: Hash[String, Integer]
puts any_size(typed)
loose = {}
loose["a"] = 2
puts total(loose)
