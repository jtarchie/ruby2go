# rbs_inline: enabled

#: (*Integer) -> Integer
def total(*ns) = ns.reduce(0) { |acc, n| acc + n }

#: (String, *Integer) -> String
def label(name, *ns) = "#{name}:#{ns.inspect}:#{ns.size}:#{ns.empty?}"

#: (*String) -> Array[String]
def collect(*parts) = parts

#: (*untyped) -> String
def kinds(*xs) = xs.map { |x| x.inspect }.join(",")

#: (?String, *Integer) -> String
def opt_then_rest(prefix = "p", *ns) = prefix + ns.map(&:to_s).join

#: (*[Integer, String]) -> String
def pairs(*ps) = ps.map { |n, s| s * n }.join("/")

#: (*Integer) -> Integer
def wrap(*xs) = total(*xs)

#: (*Integer) -> Array[Integer]
def grow(*xs)
  xs << 99
  xs
end

#: (Integer, *Integer) -> Integer
def first_plus(x, *rest) = x + rest.size

nums = [1, 2, 3] #: Array[Integer]
none = [] #: Array[Integer]
puts total, total(5), total(1, 2), total(*nums), total(*none), total(-5, 5)
puts label("x"), label("y", 1), label("z", *nums), label("w", *none)
puts collect("a", "b").inspect, collect.inspect, collect("é").inspect
words = ["p", "q"] #: Array[String]
puts collect(*words).inspect
got = collect(*words)
got << "r"
puts got.inspect, words.inspect
puts kinds, kinds(1, "a", :b, nil, 2.5, [1], true)
puts opt_then_rest, opt_then_rest("q"), opt_then_rest("r", 1, 2)
puts pairs, pairs([2, "a"], [1, "b"])
pa = [[3, "x"], [0, "y"]] #: Array[[Integer, String]]
puts pairs(*pa)
puts wrap(1, 2, 3), wrap, wrap(*nums), grow(1).inspect, grow.inspect, grow(*nums).inspect, nums.inspect
puts first_plus(0, *nums), first_plus(5), first_plus(1, 2)

puts 1, 2
puts "a", ["b", "c"], nil, 3
puts
print "x", "y", "\n"
print
puts "end"
