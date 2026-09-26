# rbs_inline: enabled

#: (untyped) -> untyped
def ident(x) = x

#: (untyped) -> untyped
def lookup(x) = x["a"]

#: (untyped) -> bool
def has_a(x) = x.key?("a")

# Narrowing an untyped value to Hash for reads (writes: hash_bug_narrowed_copy).
#: (untyped) -> String
def kind(x)
  case x
  when Hash then "hash #{x.size} #{x.keys.inspect}"
  when Array then "array #{x.size}"
  else "other #{x.inspect}"
  end
end

#: (untyped) -> Integer
def size_if_hash(x)
  if x.is_a?(Hash)
    x.size
  else
    -1
  end
end

#: (Integer) -> Integer
def inc(n) = n + 1

#: (String) -> String
def up(s) = s.upcase

#: (untyped) -> String
def describe(x) = x.inspect

#: (untyped) -> Integer
def dyn_size(x) = x.size

#: (Hash[untyped, untyped]) -> Integer
def any_size(h) = h.size

# Decision 13: an unannotated `{}` is Hash[untyped, untyped].
loose = {}
loose["a"] = 1
loose[:b] = "two"
loose[3] = [3]
loose[nil] = nil
loose[2.5] = { x: 1 }
puts loose.inspect, loose.size
puts loose["a"].inspect, loose[:b].inspect, loose[3].inspect, loose[:zz].inspect, loose[nil].inspect
puts loose.key?(nil).inspect, loose.key?("b").inspect
loose.delete(:b)
puts loose.keys.inspect

# Decision 22: literal parts with no common type join to untyped.
inferred = { "a" => 1, "b" => "x" }
puts inferred.inspect, inferred["b"].inspect
k_mixed = { 1 => "a", :b => "c" }
puts k_mixed.inspect, k_mixed[:b].inspect, k_mixed[1].inspect

typed = { "n" => 1 } #: Hash[String, Integer]
puts describe(typed), dyn_size(typed), any_size({}), any_size(loose)
puts describe({ a: [1, { b: nil }] })

cfg = { port: 8080, host: "localhost", debug: false, ratio: 0.5 } #: Hash[Symbol, untyped]
port = cfg[:port]
puts port.inspect
puts (cfg[:port] + 1).inspect
puts cfg[:host].upcase
puts cfg.fetch(:debug).inspect, cfg.fetch(:nope, "dflt").inspect
puts cfg.select { |_k, v| v }.keys.inspect
puts cfg.map { |k, v| "#{k}:#{v}" }.join(",")
puts cfg.reject { |k, _v| k == :ratio }.inspect
cfg[:port] = "now a string"
puts cfg[:port].inspect, cfg.size

# Dynamic calls on a typed hash held as untyped (decision 32).
th = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
u = ident(th)
puts u.size, u.inspect, u.to_s, u.length, u.count
puts lookup(th).inspect, has_a(th).inspect, has_a({}).inspect
puts u.fetch("b").inspect, u.fetch("zz", 0).inspect
puts u.empty?.inspect, u.include?("b").inspect, u.has_key?("zz").inspect, u.delete("zz").inspect
puts (u == th).inspect, (th == u).inspect, u.is_a?(Hash).inspect
puts kind(th), kind([1]), kind(1), kind({}), size_if_hash(th), size_if_hash("s")
# A write through the untyped alias reaches the typed hash.
u["c"] = 3
puts th.inspect

# Untyped values flow into typed parameters.
cfg2 = { port: 8080, host: "h", nested: { a: 1 } } #: Hash[Symbol, untyped]
puts inc(cfg2.fetch(:port)), up(cfg2.fetch(:host))
pv = cfg2[:port]
puts inc(pv) if pv
nested = cfg2.fetch(:nested)
puts nested.inspect, nested.size, nested[:a].inspect

# Iterating an untyped hash: each key and value keeps its own type.
mixed = { "s" => 1, 2 => :two, nil => nil, [1] => 1.5 }
mixed.each { |k, v| puts "#{k.inspect}=#{v.inspect}" }
puts mixed.select { |k, _v| k.is_a?(String) }.inspect
puts mixed.map { |k, _v| k.nil? }.inspect, mixed.count, mixed.keys.size
inferred_nil = { "a" => 1, "b" => nil }
inferred_nil.each { |k, v| puts "#{k} #{v.nil?}" }
puts inferred_nil.keys.inspect, inferred_nil.size
# Integer and Float values that are == compare equal inside Hash#==.
puts ({ "n" => 1, "s" => "x" } == { "n" => 1.0, "s" => "x" }).inspect
c1 = { "n" => 1 } #: Hash[String, untyped]
c2 = { "n" => 1.0 } #: Hash[String, untyped]
puts (c1 == c2).inspect, (c1 == { "n" => 2.0 }).inspect
# fetch on untyped values: stored false/nil come back, a false default is used.
flags = { debug: false, name: nil } #: Hash[Symbol, untyped]
puts flags.fetch(:debug).inspect, flags.fetch(:name).inspect, flags.fetch(:nope, false).inspect
