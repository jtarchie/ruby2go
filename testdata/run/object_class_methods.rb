# rbs_inline: enabled

# def self.x, inherited and virtual class methods, class objects as values, singleton(C), klass.new, self.class.

class Shape
  attr_reader :size #: Integer

  #: (Integer) -> void
  def initialize(size)
    @size = size
  end

  #: () -> String
  def self.kind = "shape"

  #: () -> Integer
  def self.sides = 0

  #: () -> String
  def self.info = "#{kind}/#{sides}"

  #: (Integer) -> Shape
  def self.build(n) = new(n + 1)

  #: (Integer) -> Shape
  def self.build_explicit(n) = self.new(n * 2)

  #: () -> String
  def describe = "#{self.class.kind}:#{self.class.name}(#{size}) sides=#{self.class.sides}"

  #: () -> Shape
  def grow = self.class.new(size + 100)
end

class Square < Shape
  def self.kind = "square"

  def self.sides = 4

  def self.info = "sq " + super
end

class Tri < Shape
  def self.sides = 3
end

class Tiny < Square
  #: () -> void
  def initialize = super(1)

  def self.kind = "tiny"
end

class Registry
  #: () -> Integer
  def self.count
    @count ||= 0
  end

  #: () -> void
  def self.bump
    @count = count + 1
  end

  #: (String) -> String
  def self.fmt(s) = "<#{s}>"
end

class SubRegistry < Registry
  def self.fmt(s) = super(s.upcase) + "!"
end

puts Shape.kind, Square.kind, Tri.kind, Tiny.kind
puts Shape.info, Square.info, Tri.info, Tiny.info
puts Shape.build(1).describe, Square.build(1).describe, Tri.build_explicit(5).describe
puts Square.new(2).grow.describe, Tri.new(0).grow.describe
puts Tiny.new.describe, Tiny.new.size.inspect

klasses = [Shape, Square, Tri] #: Array[singleton(Shape)]
klasses.each { |k| puts k.new(7).describe }
puts klasses.map(&:kind).inspect, klasses.map { |k| k.sides }.inspect
puts klasses.map(&:name).inspect, klasses.map(&:to_s).inspect, klasses.inspect
puts klasses.map { |k| k.build(0).size }.inspect

k = klasses.fetch(1)
puts k.kind, k.name, k.new(3).describe, k.info
k = klasses.fetch(2)
puts k.kind, (k == Tri).inspect, (k == Square).inspect, (k != Square).inspect

by_name = { "sq" => Square, "tri" => Tri } #: Hash[String, singleton(Shape)]
found = by_name["tri"]
puts found.kind if found
puts by_name.keys.inspect, by_name.values.inspect

s = Square.new(9)
puts s.class, s.class.name, s.class.kind, (s.class == Square).inspect, (s.class == Shape).inspect
shapes = [Shape.new(1), Square.new(2), Tri.new(3), Tiny.new] #: Array[Shape]
puts shapes.map { |x| x.class.name }.inspect
puts shapes.map { |x| x.class.sides }.inspect
puts shapes.map { |x| x.class == Square }.inspect

puts Shape.class, Square.class, Shape.class.class, Comparable.class
puts Shape.inspect, Shape.to_s, Shape.name.inspect
puts 1.class, "s".class, :sym.class, 1.5.class, [1].class, { a: 1 }.class, 1.class.class, Integer.class
puts Integer.name, String.inspect, (1.class == Integer).inspect, ("s".class == Symbol).inspect

Registry.bump
Registry.bump
SubRegistry.bump
puts Registry.count.inspect, SubRegistry.count.inspect
puts Registry.fmt("a"), SubRegistry.fmt("b"), SubRegistry.fmt("")

mods = [Shape, Comparable, Enumerable, Registry] #: Array[Module]
puts mods.map(&:name).inspect, mods.map(&:to_s).inspect, mods.inspect
klass_list = [Shape, Registry, String] #: Array[Class]
puts klass_list.map(&:name).inspect, (klass_list.first(1).fetch(0) == Shape).inspect

class Color
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  RED = Color.new("red")
  ALL = [RED, Color.new("blue")] #: Array[Color]

  #: () -> Color
  def self.default = RED

  #: (String) -> String
  def self.fmt(s) = "<#{s}>"
end

class Shade < Color
  #: (String, ?Integer) -> void
  def initialize(name, depth = 3)
    super(name)
    @depth = depth
  end

  def self.fmt(s) = super + "!"

  #: () -> String
  def self.plain = fmt("p")

  #: () -> Integer
  def depth = @depth
end

class Deep < Shade
  def initialize(name, depth = 9) = super
end

class Color
  #: () -> String
  def shout = name.upcase
end

puts Color::RED.name, Color::ALL.map(&:name).inspect, Color.default.shout
puts Shade.fmt("x"), Shade.plain, Deep.new("d").depth.inspect, Deep.new("e", 1).depth.inspect, Shade.new("s").shout

# Joins of class objects and of instances (ternary, case), class objects as Hash keys,
# and a subclass reopened without restating its superclass.
#: (bool) -> singleton(Shape)
def pick(flag) = flag ? Square : Tri

#: (Integer) -> Shape
def by_num(n)
  case n
  when 0 then Shape.new(0)
  when 1 then Square.new(1)
  else Tri.new(2)
  end
end

puts pick(true).kind, pick(false).kind, pick(true).new(4).describe, by_num(1).describe, by_num(2).class, by_num(0).describe
flag = Shape.sides == 0
kl = flag ? Square : Tri
puts kl.kind, kl.new(5).describe, kl.name
inst = flag ? Tri.new(1) : Square.new(1)
puts inst.describe

names = { Square => "sq" } #: Hash[singleton(Shape), String]
names[Tri] = "tri"
names[Square] = "SQ"
puts names[Square].inspect, names[Tri].inspect, names[Shape].inspect, names.size.inspect, names.keys.inspect
tally = {} #: Hash[singleton(Shape), Integer]
[Square, Tri, Square, Tiny].each { |c| tally[c] = tally.fetch(c, 0) + 1 }
puts tally.inspect, tally.keys.map(&:kind).inspect

class Square
  #: () -> String
  def self.extra = "reopened #{kind}"

  #: () -> Integer
  def area = size * size
end

puts Square.extra, Tiny.extra, Square.new(3).area.inspect, Tiny.new.area.inspect

# A module's own class-level ivar; `self` in a class method is the (virtual) class object;
# a class referenced before its definition; a constant holding class objects.
module Tally
  #: () -> Integer
  def self.count
    @count ||= 0
  end

  #: () -> void
  def self.bump
    @count = count + 1
  end
end

class Shape
  #: () -> singleton(Shape)
  def self.me = self

  #: () -> Maker
  def maker = Maker.new(self)
end

class Maker
  #: (Shape) -> void
  def initialize(s)
    @s = s
  end

  #: () -> String
  def made = "made #{@s.class.kind} #{@s.size}"
end

HANDLERS = [Shape, Square, Tiny]
Tally.bump
Tally.bump
puts Tally.count.inspect, Shape.me.kind, Tiny.me.kind, Tiny.me.name, Square.new(2).maker.made, Tiny.new.maker.made
puts HANDLERS.map(&:kind).inspect, HANDLERS.map { |h| h.me.sides }.inspect, HANDLERS.inspect

# Class methods taking value blocks (not iterators), with a default argument.
class Conf
  attr_accessor :port #: Integer

  #: () -> void
  def initialize
    @port = 80
  end

  #: () { (Conf) -> void } -> Conf
  def self.build
    c = new
    yield c
    c
  end

  #: (?Integer) { (Integer) -> String } -> String
  def self.fmt(n = 1) = yield(n * 2)
end

conf = Conf.build { |x| x.port = 8080 }
puts conf.port.inspect, Conf.fmt { |n| "n=#{n}" }, Conf.fmt(5) { |n| n.to_s * 2 }

# An explicit `< Object`; class-body constants built by a qualified class-method call and
# from earlier constants, in source order.
class Temp < Object
  attr_reader :deg #: Integer

  #: (Integer) -> void
  def initialize(deg)
    @deg = deg
  end

  #: (Integer) -> Temp
  def self.of(d) = new(d)

  FREEZING = Temp.of(0)
  BOILING = Temp.new(100)
  RANGE = BOILING.deg - FREEZING.deg
end

tmp = Temp.new(1)
puts Temp::FREEZING.deg.inspect, Temp::RANGE.inspect, tmp.is_a?(Object).inspect, tmp.deg.inspect

# A registry of class objects in a constant; an ivar holding a class object (a factory);
# a module's to_s used by puts and interpolation.
class Handler
  REGISTRY = {} #: Hash[String, singleton(Handler)]

  #: () -> String
  def self.tag = "base"

  #: (String, singleton(Handler)) -> void
  def self.register(name, k)
    REGISTRY[name] = k
  end

  #: (String) -> String
  def self.dispatch(name)
    k = REGISTRY[name]
    return "none" unless k
    k.new.run
  end

  #: () -> String
  def run = "run #{self.class.tag}"
end

class Posts < Handler
  def self.tag = "posts"
end

class Users < Handler
  def self.tag = "users"
end

class Factory
  #: (singleton(Handler)) -> void
  def initialize(klass)
    @klass = klass
  end

  #: () -> Handler
  def make = @klass.new

  #: () -> String
  def kind = @klass.tag
end

module Pretty
  #: () -> String
  def title = raise(NotImplementedError)

  #: () -> String
  def to_s = "pretty #{title}"
end

class Doc
  include Pretty

  #: () -> String
  def title = "d"
end

Handler.register("p", Posts)
Posts.register("u", Users)
puts Handler.dispatch("p"), Users.dispatch("u"), Handler.dispatch("x"), Handler::REGISTRY.keys.inspect, Handler::REGISTRY.size.inspect
fa = Factory.new(Users)
puts fa.make.run, fa.kind, Factory.new(Handler).make.run
puts Doc.new, "#{Doc.new}!", Doc.new.to_s.size.inspect

# respond_to? on class objects: class methods, subclass-only class methods through singleton(C), new, name.
class Blob
  #: () -> String
  def self.kind = "blob"

  #: () -> String
  def area = "a"
end

class SubBlob < Blob
  #: () -> String
  def self.only_sub = "sub"
end

blobs = [Blob, SubBlob] #: Array[singleton(Blob)]
puts Blob.respond_to?(:kind).inspect, Blob.respond_to?(:area).inspect, SubBlob.respond_to?(:only_sub).inspect, Blob.respond_to?(:only_sub).inspect
puts blobs.map { |k| k.respond_to?(:only_sub) }.inspect, Blob.new.respond_to?(:kind).inspect, Blob.respond_to?(:new).inspect, Blob.respond_to?(:name).inspect

# A class's own name/to_s override: to_s and inspect do not call a custom name.
class Widget
  #: () -> String
  def self.name = "CustomWidget"
end

class Gadget
  #: () -> String
  def self.to_s = "GadgetClass"
end

puts Widget.name, Widget, Widget.inspect, "#{Widget}", [Widget].inspect, Widget.new.class.name
puts Gadget.name, Gadget, Gadget.inspect, "#{Gadget}"
