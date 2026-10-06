# rbs_inline: enabled

# A lambda with a `*rest` parameter takes any number of arguments past its
# leading ones. rb2go types it from its calls (no annotation needed) and
# compiles it to a variadic Go func: `->(label, *nums)` called with Integers
# is a `func(String, ...Integer) String`, so each call is a direct Go call.

report = ->(label, *nums) { "#{label}: #{nums.sum} over #{nums.size}" }
puts report.call("none")
puts report.call("one", 4)
puts report.("three", 1, 2, 3)

# Mixed argument types join to a union (decision 150): Integer | Float | String.
fmt = -> f, *args { f % args }
puts fmt.call("%d apples", 3)
puts fmt.call("%.2f kg of %s", 1.5, "pears")

# A bare `*` drops the rest; arity is negative, as in MRI.
first = ->(a, *) { a }
puts first.call(:x, :y, :z)
p [report.arity, first.arity]
