# rbs_inline: enabled

# Struct-class values and keys, tuple keys, and hashes held by objects,
# constants, arrays and other hashes.

class Animal
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: () -> String
  def speak = "..."
end

class Dog < Animal
  #: () -> String
  def speak = "woof"
end

class Registry
  attr_reader :items #: Hash[String, Animal]

  #: () -> void
  def initialize
    @items = {}
  end

  #: (Animal) -> self
  def add(a)
    @items[a.name] = a
    self
  end

  #: () -> Array[String]
  def names = @items.keys

  # Values dispatch virtually: a Dog stored as an Animal still says woof.
  #: () -> String
  def voices = @items.map { |k, v| "#{k}:#{v.speak}" }.join(" ")
end

class Tag
  #: (String) -> void
  def initialize(s)
    @s = s
  end

  #: () -> String
  def inspect = "<#{@s}>"
end

Point = Data.define(:x, :y) #: [Integer, Integer]
Pair = Struct.new(:a, :b) #: [String, Integer]

LIMITS = { "low" => 1, "high" => 9 } #: Hash[String, Integer]

#: (String) -> Integer
def limit(name) = LIMITS.fetch(name, 0)

# A default `{}` is a fresh hash on every call, as in Ruby.
#: (?Hash[String, Integer]) -> Integer
def bump(h = {})
  h["x"] = (h["x"] || 0) + 1
  h.size * 10 + (h["x"] || 0)
end

#: (Array[String]) -> Hash[String, Integer]
def lengths(ws)
  out = {} #: Hash[String, Integer]
  ws.each { |w| out[w] = w.size }
  out
end

r = Registry.new
r.add(Animal.new("cat")).add(Dog.new("rex"))
puts r.names.inspect, r.voices
# The reader returns the same hash, so writes through it show up.
r.items["bob"] = Dog.new("bob")
puts r.items.size, r.names.inspect
puts r.items["rex"]&.speak.inspect, r.items["zz"]&.speak.inspect

# Plain objects are keys by identity, in MRI too.
a1 = Animal.new("x")
a2 = Animal.new("x")
ids = {} #: Hash[Animal, Integer]
ids[a1] = 1
ids[a2] = 2
ids[a1] = 3
puts ids.size, ids[a1].inspect, ids[a2].inspect, ids.key?(Animal.new("x")).inspect
same = Point.new(x: 1, y: 1)
by_id = { same => 1 } #: Hash[Point, Integer]
puts by_id[same].inspect

# Tuple keys compare by value.
tk = {} #: Hash[[Integer, String], Integer]
tk[[1, "a"]] = 1
tk[[1, "a"]] = 2
tk[[2, "a"]] = 3
puts tk.size, tk[[1, "a"]].inspect, tk[[1, "b"]].inspect, tk.inspect
pairs = [[1, "a"], [1, "a"], [2, "b"]]
puts pairs.tally.inspect
words = %w[apple avocado banana blueberry cherry]
puts words.group_by { |w| [w.size, w[0]] }.inspect

# Keys built at run time find literal keys.
sk = { "ab" => 1, ab: 2 } #: Hash[untyped, Integer]
x = "a"
puts sk[x + "b"].inspect, sk["#{x}b"].inspect, sk[(x + "b").to_sym].inspect, sk[x].inspect

# Values with their own inspect.
pts = { "o" => Point.new(x: 0, y: 0), "p" => Point.new(x: 1, y: -2) } #: Hash[String, Point]
puts pts.inspect, pts.values.map(&:x).inspect
puts (pts == { "o" => Point.new(x: 0, y: 0), "p" => Point.new(x: 1, y: -2) }).inspect
puts (pts == { "o" => Point.new(x: 0, y: 1), "p" => Point.new(x: 1, y: -2) }).inspect
pr = { k: Pair.new("s", 1) } #: Hash[Symbol, Pair]
puts pr.inspect, pr[:k]&.b.inspect
tg = { Tag.new("k") => Tag.new("v") } #: Hash[Tag, Tag]
puts tg.inspect
mixed = { 1 => Tag.new("t"), 2 => Point.new(x: 3, y: 4) } #: Hash[Integer, untyped]
puts mixed.inspect

puts limit("high"), limit("mid"), LIMITS.size, LIMITS.inspect
puts bump, bump
shared = { "y" => 5 } #: Hash[String, Integer]
puts bump(shared), bump(shared), shared.inspect

# An array of hashes: hashes are shared, so each's writes stick.
rows = [{ "n" => 3, "k" => 1 }, { "n" => 1, "k" => 2 }, { "n" => 2, "k" => 3 }] #: Array[Hash[String, Integer]]
puts rows.sort_by { |row| row.fetch("n") }.map { |row| row.fetch("k") }.inspect
rows.each { |row| row["n"] = row.fetch("n") + 10 }
puts rows.inspect, rows.select { |row| row.fetch("k").odd? }.size
puts rows.max_by { |row| row.fetch("n") }.inspect
people = [{ name: "a", age: 30 }, { name: "b", age: 20 }] #: Array[Hash[Symbol, untyped]]
puts people.map { |p| p.fetch(:name) }.inspect

# A hash of hashes: buckets created on demand, then written through fetch.
reg = {} #: Hash[String, Hash[String, Integer]]
%w[x y x].each_with_index do |name, i|
  inner = reg[name]
  if inner
    inner["n#{i}"] = i
  else
    reg[name] = { "n#{i}" => i }
  end
end
puts reg.inspect
reg.fetch("x")["z"] = 99
puts reg["x"].inspect, reg.map { |k, v| [k, v.size] }.inspect, reg.values.map(&:size).inspect

# A hash returned from a method.
lengths(%w[aa b]).each { |k, v| puts "#{k}=#{v}" }
puts lengths([]).empty?.inspect
if (n = lengths(%w[q]).fetch("q", 0)) > 0
  puts "got #{n}"
end
zz = lengths(%w[zz y])["zz"]
puts zz + 1 if zz

# Hashes held by ivars, accessors, Struct members and optional locals.
class Memo
  # @rbs @cache: Hash[Integer, Integer]

  #: () -> void
  def initialize
    @cache = {}
  end

  #: (Integer) -> Integer
  def fib(n)
    c = @cache[n]
    return c if c

    r = n < 2 ? n : fib(n - 1) + fib(n - 2)
    @cache[n] = r
  end

  #: () -> Integer
  def cached = @cache.size
end

class Settings
  attr_accessor :opts #: Hash[String, String]

  #: () -> void
  def initialize
    @opts = { "mode" => "fast" }
  end
end

Rec = Struct.new(:name, :tags) #: [String, Hash[String, Integer]]

m = Memo.new
puts m.fib(40), m.cached
s = Settings.new
s.opts["color"] = "red"
puts s.opts.inspect
before = s.opts
s.opts = { "x" => "y" }
puts s.opts.inspect, before.inspect
rec = Rec.new("n", { "t" => 1 })
rec.tags["u"] = 2
puts rec.inspect, rec.tags.size
lazy = nil #: Hash[String, Integer]?
lazy ||= {}
lazy["k"] = 1
lazy ||= { "never" => 0 }
puts lazy.inspect
maybe = nil #: Hash[String, Integer]?
puts maybe&.size.inspect, maybe.inspect
maybe = { "q" => 1 }
puts maybe&.size.inspect, maybe&.fetch("q").inspect
pair = [{ "in" => 1 }, 2] #: [Hash[String, Integer], Integer]
pair[0]["in2"] = 3
puts pair[0].inspect, pair[1]
