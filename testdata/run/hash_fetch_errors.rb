# rbs_inline: enabled

#: (Hash[String, Integer], String) -> String
def safe_fetch(h, k)
  h.fetch(k).to_s
rescue KeyError => e
  "rescued: #{e.message}"
end

h = { "a" => 1 } #: Hash[String, Integer]
puts safe_fetch(h, "a"), safe_fetch(h, "zz"), safe_fetch(h, ""), safe_fetch(h, "é\n")

begin
  { a: 1 }.fetch(:nope)
rescue KeyError => e
  puts e.message, e.class
end

ints = { 1 => "x" } #: Hash[Integer, String]
begin
  ints.fetch(-7)
rescue => e
  puts e.message, e.class, e.is_a?(IndexError).inspect, e.is_a?(StandardError).inspect
end

# A nil lookup raises NoMethodError when a method is called on it (decision 20).
puts h["a"].succ
begin
  h["zz"].succ
rescue NoMethodError => e
  puts "rescued #{e.class}"
end

flags = { "on" => true, "off" => false } #: Hash[String, bool]
puts flags["off"].inspect, flags["missing"].inspect
puts flags.fetch("off").inspect, flags.fetch("on", false).inspect, flags.fetch("off", true).inspect
puts flags.fetch("missing", true).inspect, flags.fetch("missing", false).inspect

# KeyError messages inspect the key, whatever its type.
#: (Hash[untyped, Integer], untyped) -> String
def try_fetch(h, k)
  h.fetch(k).inspect
rescue KeyError => e
  e.message
end

u = { "a" => 1 } #: Hash[untyped, Integer]
puts try_fetch(u, nil), try_fetch(u, 1.5), try_fetch(u, :"a b"), try_fetch(u, [1, "x"]), try_fetch(u, "a")
tk = { [1, "a"] => 1 } #: Hash[[Integer, String], Integer]
begin
  tk.fetch([2, "b"])
rescue KeyError => e
  puts e.message
end
sk = { a: 1 } #: Hash[Symbol, Integer]
begin
  sk.fetch(:b?)
rescue KeyError => e
  puts e.message
end
# KeyError is an IndexError.
begin
  u.fetch(:zz)
rescue IndexError => e
  puts "#{e.class}: #{e.message}"
end
x = u.fetch("zz") rescue -1
puts x

# An uncaught KeyError exits 1 after flushing what was printed.
puts "before"
ints.fetch(42)
puts "unreached"
