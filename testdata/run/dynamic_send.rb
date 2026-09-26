# rbs_inline: enabled

class Greeter
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: (?String) -> String
  def greet(greeting = "hello") = "#{greeting}, #{name}"

  #: (String, ?String, ?String) -> String
  def wrap(body, left = "[", right = "]") = left + body + right

  #: (*String) -> String
  def shout(*words) = words.map(&:upcase).join(" ")

  #: (Integer, *Integer) -> Integer
  def sum(first, *rest) = rest.reduce(first) { |acc, x| acc + x }

  #: (String?) -> String
  def maybe_name(s) = s || "anon"

  #: (String) -> String
  def hi(other) = "hi " + other

  #: () -> Integer?
  def lucky = nil

  #: () -> Integer?
  def unlucky = 13

  #: () -> void
  def announce
    puts "announcing #{name}"
  end

  #: () -> Greeter
  def twin = Greeter.new(name + "2")

  #: () -> Array[String]
  def letters = name.chars

  #: ([Integer, String]) -> String
  def pair_text(t) = "#{t[0]}/#{t[1]}"

  #: (singleton(Greeter)) -> String
  def class_text(k) = k.name

  #: () -> self
  def itself_again = self

  #: (bool) -> bool
  def flip(x) = !x

  #: (Float, Symbol) -> String
  def mixed(f, s) = "#{f}:#{s}"

  #: (Array[String]) -> Integer
  def count_words(words) = words.size

  private

  #: () -> String
  def secret = "shh"
end

class Ghost
  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = "ghost #{name}(#{args.map { |a| a.inspect }.join(", ")})"

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("g")
end

class Robot
  #: () -> String
  def greet = "BEEP"
end

module Tools
end

module Labeled
  #: () -> String
  def label = "labeled #{kind}"

  #: () -> String
  def kind = "?"
end

class Vehicle
  #: () -> String
  def wheels = "4 wheels"

  #: () -> String
  def self.fleet = "fleet"
end

class Car < Vehicle
  include Labeled

  #: () -> String
  def kind = "car"
end

class Counter
  attr_accessor :count #: Integer
  attr_accessor :note #: String?

  #: () -> void
  def initialize
    @count = 0
    @note = nil
  end

  #: (Integer?) -> String
  def opt(n) = n.nil? ? "none" : "n=#{n}"

  #: (bool) -> String
  def yn(b) = b ? "yes" : "no"

  #: (Float) -> Float
  def half(f) = f / 2
end

class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (untyped) -> bool
  def ==(other) = other.is_a?(Pt) && other.x == x
end

#: (untyped) -> untyped
def ident(v) = v

g = ident(Greeter.new("ada")) #: untyped
ghost = ident(Ghost.new) #: untyped

puts "-- the same call reaches each receiver's own method"
[g, ghost, ident(Robot.new)].each do |thing|
  puts thing.greet
  puts thing.respond_to?(:greet), thing.respond_to?(:zap), thing.respond_to?(:gone), thing.respond_to?(:to_s)
end

puts "-- optional, rest and nilable parameters"
puts g.greet, g.greet("hey"), g.greet("")
puts g.wrap("x"), g.wrap("x", "<"), g.wrap("x", "<", ">"), g.wrap("")
puts g.shout.inspect, g.shout("a"), g.shout("a", "b", "ünï")
puts g.sum(1), g.sum(1, 2, 3), g.sum(-5, 5)
puts g.maybe_name(nil), g.maybe_name("bob")

puts "-- parameter types checked at the boundary"
puts g.pair_text([7, "seven"]), g.class_text(Greeter), g.itself_again.name, g.flip(true).inspect, g.flip(false).inspect
puts g.mixed(-0.5, :sym), g.mixed(1e20, :"a b"), g.count_words(["a", "b", "c"]), g.count_words(["ü"])

puts "-- results cross back as untyped values"
puts g.lucky.inspect, g.unlucky.inspect, g.lucky.nil?.inspect, (g.unlucky + 1).inspect
puts g.announce.inspect
puts g.twin.name, g.twin.twin.greet("yo"), g.letters.inspect, g.letters.size
puts g.name.upcase, g.name.size, (g.name + "!").inspect

puts "-- dynamic calls on untyped primitives"
n = ident(7) #: untyped
s = ident("héllo") #: untyped
a = ident([3, 1, 2]) #: untyped
h = ident({ "k" => 1 }) #: untyped
puts (n + 1).inspect, (n * n).inspect, (n > 3).inspect, n.even?.inspect, n.to_s.inspect, (n - 10).abs.inspect
puts s.upcase, s.size, s.reverse, s.include?("é").inspect, s.start_with?("hé").inspect, s[0].inspect
puts a.size, a.sort.inspect, a.first(2).inspect, a.include?(2).inspect, a.last.inspect, a.empty?.inspect, a[1].inspect
puts h["k"].inspect, h["zz"].inspect, h.size, h.key?("k").inspect, h.key?("q").inspect

puts "-- send / public_send with literal and computed names"
puts g.send(:greet, "yo"), g.public_send(:name), g.send(:wrap, "m", "(", ")")
["greet", "shout", "name", "lucky"].each { |m| puts g.send(m).inspect }
[:greet, :name].each { |m| puts g.public_send(m).inspect }
puts g.send("sum", 1, 2).inspect, ghost.send("glide", 1).inspect, ghost.send(:anything).inspect
puts ["name", "fly", :greet, "secret", "to_s", "gone"].map { |m| g.respond_to?(m) }.inspect
puts ["glow", "fly"].map { |m| ghost.respond_to?(m) }.inspect

puts "-- method_missing receives unknown names and their arguments"
puts ghost.walk, ghost.fly(1, "two", :three, [4], 5.0), ghost.greet("x")

puts "-- class objects"
klass = Object.const_get("Greeter")
puts klass.new("bob").greet("hey"), klass.name, klass.inspect
[Greeter, Robot].each { |k| puts k.name }

puts "-- errors: arity (ArgumentError), MRI's messages"
begin
  g.greet("a", "b", "c")
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end
begin
  g.wrap
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end
begin
  g.sum
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end
begin
  g.name(1)
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end
begin
  g.hi
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end

puts "-- errors: argument types (TypeError)"
begin
  g.hi(5)
rescue TypeError => e
  puts e.class
end
begin
  g.wrap(1)
rescue TypeError => e
  puts e.class
end

puts "-- errors: missing methods (NoMethodError), MRI's messages"
begin
  g.fly
rescue NoMethodError => e
  puts e.message
end
begin
  ident(nil).greet
rescue NoMethodError => e
  puts e.message
end
begin
  ident(true).greet
rescue NoMethodError => e
  puts e.message
end
begin
  ident(false).greet(1)
rescue NoMethodError => e
  puts e.message
end
begin
  ident(5).greet
rescue NoMethodError => e
  puts e.message
end
begin
  ident("str").greet
rescue NoMethodError => e
  puts e.message
end
begin
  ident([1]).greet
rescue NoMethodError => e
  puts e.message
end
begin
  ident(:sym).greet
rescue NoMethodError => e
  puts e.message
end
begin
  ident(Robot).greet
rescue NoMethodError => e
  puts e.message
end
begin
  ident(Tools).greet
rescue NoMethodError => e
  puts e.message
end
begin
  g.send("fly", 1)
rescue NoMethodError => e
  puts e.message
end
begin
  g.secret
rescue NoMethodError => e
  puts e.class
end

puts "-- errors: a computed name must be a Symbol or String (TypeError)"
[1, 2.5, nil, [1]].each do |bad|
  begin
    g.send(bad)
  rescue TypeError => e
    puts e.message
  end
  begin
    puts g.respond_to?(bad)
  rescue TypeError => e
    puts e.message
  end
end

puts "-- respond_to? on untyped nil, primitives and class objects"
puts ident(nil).respond_to?(:greet), ident(nil).respond_to?(:inspect), ident(5).respond_to?(:greet), ident(5).respond_to?(:even?)
puts ident(nil).respond_to?(:nil?), ident(Greeter).respond_to?(:greet), ident(Greeter).respond_to?(:name), ident(Greeter).respond_to?(:new)
puts ident("s").respond_to?(:upcase), ident([]).respond_to?(:each), ident({}).respond_to?(:key?), ident(:s).respond_to?(:to_sym)

puts "-- inherited, included and Comparable methods reached dynamically"
car = ident(Car.new)
puts car.label, car.wheels, car.kind, car.respond_to?(:label), car.respond_to?(:wheels), car.respond_to?(:fleet)
puts ident(3).between?(1, 5), ident("b").clamp("a", "c"), ident([3, 1, 2]).min, ident([3, 1, 2]).max
puts ident(1.5).round, ident(-3).abs, ident("a,b").split(",").inspect

puts "-- class objects through untyped: new and class methods"
kv = ident(Vehicle)
puts kv.new.wheels, kv.fleet, kv.name, ident(Car).fleet, ident(Car).new.is_a?(Vehicle)
begin
  ident(Greeter).new
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end

puts "-- attribute writers, []= and typed parameters through a dynamic call"
cu = ident(Counter.new)
cu.count = 5
puts cu.count
cu.note = "memo"
puts cu.note.inspect
cu.note = nil
puts cu.note.inspect
puts cu.opt(nil), cu.opt(3), cu.yn(true), cu.yn(false), cu.half(3.0)
arr = ident([1, 2, 3])
arr[0] = 10
puts arr.inspect
hh = ident({ "k" => 1 })
hh["z"] = 2
puts hh.inspect

puts "-- nil, true and false arguments: MRI's TypeError wording"
[nil, true, false].each do |bad|
  begin
    g.hi(bad)
  rescue TypeError => e
    puts e.message
  end
end

puts "-- == on untyped uses the receiver's own =="
pu = ident(Pt.new(1))
puts pu == Pt.new(1), pu == Pt.new(2), pu != Pt.new(1), pu == 1, Pt.new(3) == ident(Pt.new(3))

puts "-- send with literal names on untyped receivers"
puts g.send(:greet, "yo"), g.public_send(:wrap, "p"), g.__send__(:name)
puts ident(5).send(:+, 1), ident("ab").public_send(:upcase), ident([2, 1]).send(:sort).inspect
puts g.respond_to?("greet"), g.respond_to?(:secret), ident("s").respond_to?("upcase")
begin
  g.public_send(:secret)
rescue NoMethodError => e
  puts e.class
end

puts "-- respond_to? with a computed name on a typed receiver"
robot = Robot.new
names = ["greet", "fly", :greet] #: Array[untyped]
puts names.map { |m| robot.respond_to?(m) }.inspect

puts "-- an uncaught NoMethodError exits 1"
ident(Robot.new).fly
puts "not reached"
