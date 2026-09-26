# rbs_inline: enabled

class Config
  #: () -> void
  def initialize
    @values = { "color" => "blue", "size" => "L", "empty" => "" } #: Hash[String, String]
  end

  #: () -> String
  def to_s = "config"

  #: () -> String
  def color_twice = color + color

  #: (Symbol, *untyped) -> String
  def method_missing(name, *args)
    value = @values[name.to_s]
    return value if value
    "(no #{name}: #{args.inspect})"
  end

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = @values.key?(name.to_s)

  private

  #: () -> String
  def secret = "s3cret"
end

class StrictConfig < Config
  #: () -> String
  def size = "overridden"
end

class Recorder
  #: () -> void
  def initialize
    @calls = [] #: Array[String]
  end

  #: () -> Array[String]
  def calls = @calls

  #: (Symbol, *untyped) -> Integer
  def method_missing(name, *args)
    @calls << "#{name}(#{args.map { |a| a.inspect }.join(", ")})"
    @calls.size
  end
end

class Plain
  #: () -> String
  def hello = "hi"

  #: (Integer) -> Integer
  def twice(n) = n * 2

  private

  #: () -> String
  def hidden = "hidden"
end

class SelfCaller
  #: () -> String
  def hi = "hi"

  #: () -> String
  def call_hi = self.send(:hi)

  #: () -> bool
  def can_hi = respond_to?(:hi)

  #: () -> bool
  def can_bye = self.respond_to?(:bye)

  #: (String) -> String
  def call_named(n) = self.send(n)
end

class Base
  #: () -> String
  def name = "base"

  #: () -> bool
  def can_extra? = respond_to?(:extra)
end

class Derived < Base
  #: () -> String
  def extra = "extra"
end

class Fallback
  #: (Symbol, *untyped) -> String
  def method_missing(name, *args) = "fallback #{name} #{args.size}"

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = name.to_s.end_with?("_x")

  #: () -> String
  def real = "real"
end

class FallbackChild < Fallback
  #: () -> String
  def own = "own #{zzz_x}"
end

module Chatty
  #: () -> String
  def chat = "chat #{unknown_thing}"
end

class Talker
  include Chatty

  #: (Symbol, *untyped) -> String
  def method_missing(name, *args) = "talker #{name}"
end

class Shy
  #: () -> String
  def hi = "hi"

  private

  #: (Symbol, *untyped) -> String
  def method_missing(name, *args) = "shy #{name} #{args.inspect}"

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = name == :maybe
end

class Proxy
  #: (untyped) -> void
  def initialize(target)
    @target = target #: untyped
  end

  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = @target.send(name)

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = @target.respond_to?(name)
end

#: (untyped) -> untyped
def ident(v) = v

puts "-- unknown methods go to method_missing, typed by its signature"
c = Config.new
puts c.color, c.size, c.empty.inspect, c.weight, c.weight(1, "two", nil), c.color.upcase
puts c.color_twice, c.to_s, c.missing?.inspect, c.set!.inspect
s = StrictConfig.new
puts s.size, s.color, s.nothing(:x)

puts "-- method_missing returning a counter"
r = Recorder.new
puts r.first, r.second(1), r.third("a", [2], nil, 1.5, :sym)
puts r.calls.inspect
total = r.one + r.two
puts total

puts "-- respond_to? folds to a constant or asks respond_to_missing?"
puts c.respond_to?(:color), c.respond_to?(:weight), c.respond_to?(:to_s), c.respond_to?(:color_twice)
puts c.respond_to?("size"), c.respond_to?("nope"), c.respond_to?(:empty)
puts c.respond_to?(:secret), c.respond_to?(:initialize), c.respond_to?(:respond_to_missing?)
puts s.respond_to?(:size), s.respond_to?(:weight), s.respond_to?(:inspect)
puts r.respond_to?(:anything), r.respond_to?(:calls)
p1 = Plain.new
puts p1.respond_to?(:hello), p1.respond_to?(:twice), p1.respond_to?(:bye), p1.respond_to?(:hidden), p1.respond_to?(:nil?)
puts 5.respond_to?(:even?), 5.respond_to?(:upcase), "s".respond_to?(:upcase), "s".respond_to?(:even?)
puts [1].respond_to?(:each), [1].respond_to?(:nope), 1.5.respond_to?(:floor), :sym.respond_to?(:to_proc_nope)

puts "-- send and public_send with literal names are ordinary calls"
puts p1.send(:hello), p1.public_send(:hello), p1.send(:twice, 21), p1.public_send(:twice, -4), p1.__send__(:hello)
puts p1.send(:hidden), c.send(:secret)
puts c.send(:color), c.public_send(:size), c.send(:weight, 3)
puts p1.send("hello"), p1.send("twice", 5)
puts 5.send(:+, 3), "abc".send(:upcase), [3, 1, 2].send(:sort).inspect, 10.public_send(:-, 20)

puts "-- send and respond_to? on self"
sc = SelfCaller.new
puts sc.call_hi, sc.can_hi, sc.can_bye, sc.call_named("hi").inspect

puts "-- respond_to? on a base-typed value holding a subclass"
#: (Base) -> String
def probe(b) = "#{b.name} #{b.respond_to?(:name)} #{b.respond_to?(:extra)} #{b.respond_to?(:zzz)}"
puts probe(Base.new), probe(Derived.new)

puts "-- inherited method_missing and respond_to_missing?"
fc = FallbackChild.new
puts fc.own, fc.foo_x, fc.bar(1, 2), fc.real
puts fc.respond_to?(:foo_x), fc.respond_to?(:foo), fc.respond_to?(:own), fc.respond_to?(:real)
fu = ident(fc)
puts fu.anything(1), fu.own, fu.respond_to?(:q_x), fu.respond_to?(:q)
puts fc.send(:dyn_x, 3), fc.public_send(:dyn_y), fc.method_missing(:direct, 1)

puts "-- a module method calling an unknown name reaches the includer's method_missing"
puts Talker.new.chat

puts "-- a private method_missing still handles typed and untyped calls"
shy = Shy.new
puts shy.whatever, shy.other(1), shy.respond_to?(:maybe), shy.respond_to?(:nope), shy.respond_to?(:hi)
shu = ident(shy)
puts shu.whatever, shu.other(2), shu.respond_to?(:maybe), shu.respond_to?(:nope), shu.hi
puts shu.send(["dyn"].first(1)[0] || "", 3)
puts shy.respond_to?(:method_missing), shy.respond_to?(:respond_to_missing?)

puts "-- method_missing forwarding to an untyped target"
px = Proxy.new("héllo")
puts px.upcase, px.size, px.reverse, px.respond_to?(:upcase), px.respond_to?(:fly)
begin
  px.fly
rescue NoMethodError => e
  puts e.message
end

puts "-- respond_to? on self in a base class sees the subclass's methods"
puts Base.new.can_extra?, Derived.new.can_extra?
