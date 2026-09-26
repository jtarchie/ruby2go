# rbs_inline: enabled

a = { "x" => 1, "y" => 2 } #: Hash[String, Integer]
b = { "y" => 2, "x" => 1 } #: Hash[String, Integer]
c = { "x" => 1 } #: Hash[String, Integer]
d = { "x" => 1, "y" => 3 } #: Hash[String, Integer]
f = { "x" => 1, "z" => 2 } #: Hash[String, Integer]

# Hash#== ignores insertion order but compares sizes, keys and values.
puts (a == b).inspect, (b == a).inspect, (a == a).inspect
puts (a == c).inspect, (c == a).inspect, (a == d).inspect, (a == f).inspect
puts (c == { "x" => 1 }).inspect, (c == { "x" => 2 }).inspect
puts (a == 1).inspect, (a == nil).inspect, (a == [1]).inspect, (a == "x").inspect

# equal? is identity, not contents.
puts a.equal?(a).inspect, a.equal?(b).inspect
alias_a = a
puts alias_a.equal?(a).inspect

e1 = {} #: Hash[String, Integer]
e2 = {} #: Hash[String, Integer]
puts (e1 == e2).inspect, (e1 == a).inspect

# Values compare with ==, so nested arrays and hashes compare by contents.
n1 = { "k" => [1, 2], "j" => [] } #: Hash[String, Array[Integer]]
n2 = { "j" => [], "k" => [1, 2] } #: Hash[String, Array[Integer]]
n3 = { "k" => [2, 1], "j" => [] } #: Hash[String, Array[Integer]]
puts (n1 == n2).inspect, (n1 == n3).inspect
hh1 = { "o" => { "i" => 1 } } #: Hash[String, Hash[String, Integer]]
hh2 = { "o" => { "i" => 1 } } #: Hash[String, Hash[String, Integer]]
hh3 = { "o" => { "i" => 2 } } #: Hash[String, Hash[String, Integer]]
puts (hh1 == hh2).inspect, (hh1 == hh3).inspect

# Equality follows mutation.
b["y"] = 3
puts (a == b).inspect, (b == d).inspect
b.delete("y")
puts (b == c).inspect

sa = { a: 1, b: "two" } #: Hash[Symbol, untyped]
sb = { b: "two", a: 1 } #: Hash[Symbol, untyped]
sc = { a: 1, b: "TWO" } #: Hash[Symbol, untyped]
puts (sa == sb).inspect, (sa == sc).inspect

u1 = { 1 => "x", "k" => :s }
u2 = { "k" => :s, 1 => "x" }
puts (u1 == u2).inspect, (u1 == { 1 => "x" }).inspect

# A Hash as an array element compares by contents too.
puts ([{ "x" => 1 }] == [{ "x" => 1 }]).inspect, ([c] == [a]).inspect

# Plain-object values compare by identity (their ==), as in MRI.
# (Obj has an ivar: ivar-less instances are hash_bug_empty_class_keys.)
class Obj
  #: () -> void
  def initialize
    @tag = 0
  end
end
o1 = Obj.new
o2 = Obj.new
puts ({ "o" => o1 } == { "o" => o1 }).inspect, ({ "o" => o1 } == { "o" => o2 }).inspect
# Array#include? finds a hash by contents; equal? stays identity.
puts [c].include?({ "x" => 1 }).inspect, [c].include?({ "x" => 9 }).inspect
puts c.equal?(c.merge({})).inspect, (c == c.merge({})).inspect
# A select that keeps everything is equal but not identical.
puts (a == a.select { |_k, _v| true }).inspect, a.equal?(a.select { |_k, _v| true }).inspect
