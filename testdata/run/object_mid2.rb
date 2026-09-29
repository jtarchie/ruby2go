# rbs_inline: enabled

# self.class inside a module method names the including object's class
module Describe
  #: () -> String
  def kind = self.class.name

  #: () -> String
  def label = "#{self.class}!"
end

class Foo
  include Describe
end

class Bar < Foo
end

puts Foo.new.kind, Bar.new.kind, Bar.new.label

# a module as an element type dispatches to each includer's override
module Printable
  #: () -> String
  def title = raise(NotImplementedError)

  #: () -> String
  def to_s = "P(#{title})"
end

class Doc
  include Printable

  #: () -> String
  def title = "doc"
end

class Memo
  include Printable

  #: () -> String
  def title = "memo"
end

items = [Doc.new, Memo.new] #: Array[Printable]
puts items.map { |i| i.title }.inspect
puts items.map(&:to_s).inspect

# != uses a user-defined ==, and Struct's generated one
class NeVec
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (NeVec) -> bool
  def ==(o) = x == o.x
end

NePoint = Struct.new(:x, :y) #: [Integer, Integer]

a = NeVec.new(1)
puts (a == NeVec.new(1)).inspect, (a != NeVec.new(1)).inspect, (a != NeVec.new(2)).inspect
puts (NePoint.new(1, 2) != NePoint.new(1, 2)).inspect, (NePoint.new(1, 2) != NePoint.new(2, 1)).inspect

# a subclass may override with a different arity and a narrower return type
class Base
  #: (Integer) -> String
  def f(n) = "base #{n}"

  #: () -> Base
  def me = self

  #: () -> String
  def name = "base"
end

class Sub < Base
  #: () -> String
  def f = "sub"

  #: () -> Sub
  def me = self

  def name = "sub"

  #: () -> String
  def only = "only"
end

puts Base.new.f(1), Sub.new.f
puts Base.new.me.name, Sub.new.me.name, Sub.new.me.only

# private attr_reader/attr_accessor are callable on self, including self.x =
class Vault
  #: () -> void
  def initialize
    @pin = 1234
    @hits = 0
  end

  #: () -> String
  def open
    self.hits = hits + 1
    "pin has #{pin.to_s.size} digits, hit #{hits}"
  end

  private

  attr_reader :pin #: Integer
  attr_accessor :hits #: Integer
end

v = Vault.new
puts v.open, v.open

# a bare `private` does not reach `def self.x`
class PrivCounter
  #: () -> String
  def self.shown = "shown"

  private

  #: () -> String
  def self.hidden = "still public"

  #: () -> String
  def inst = "private instance"
end

puts PrivCounter.shown, PrivCounter.hidden

# reopening with `< ::A` or `class M::K < M::Base` is the same superclass
class A
end

class C < A
  #: () -> String
  def one = "one"
end

class C < ::A
  #: () -> String
  def two = "two"
end

module M
  class Base
  end

  class K < Base
  end
end

class M::K < M::Base
  #: () -> String
  def three = "three"
end

puts C.new.one, C.new.two, M::K.new.three

# a singleton(T) value converts to Class/Module parameters and Hash[Class, _] keys
class Shape
end

class Square < Shape
end

#: (Class) -> String
def cname(k) = k.name

#: (Module) -> String
def mname(k) = k.name

ks = [Square, Shape] #: Array[singleton(Shape)]
puts cname(Square), mname(Square)
puts cname(ks.fetch(0)), mname(ks.fetch(1))
counts = {} #: Hash[Class, Integer]
[Square, Shape, Square].each { |k| counts[k] = counts.fetch(k, 0) + 1 }
puts counts.inspect

# a Struct.new block can override initialize/inspect and call super
Point = Struct.new(:x, :y) do
  #: (Integer, ?Integer) -> void
  def initialize(x, y = 0)
    super(x, y)
  end

  #: () -> Integer
  def sum = x + y
end #: [Integer, Integer]

Pair = Struct.new(:a, :b) do
  #: () -> String
  def inspect = "P" + super
end #: [Integer, Integer]

puts Point.new(1).inspect, Point.new(1, 2).sum.inspect
puts Pair.new(1, 2).inspect

# Struct/Data == is false across a subclass, true within one
EqPoint = Struct.new(:x, :y) #: [Integer, Integer]

class EqPoint3 < EqPoint
end

Val = Data.define(:v) #: [Integer]

class SubVal < Val
end

puts (EqPoint.new(1, 2) == EqPoint3.new(1, 2)).inspect
puts (EqPoint3.new(1, 2) == EqPoint.new(1, 2)).inspect
puts (EqPoint3.new(1, 2) == EqPoint3.new(1, 2)).inspect
puts (Val.new(1) == SubVal.new(1)).inspect
puts (SubVal.new(1) == Val.new(1)).inspect
puts SubVal.new(1).inspect, SubVal.new(2).with(v: 3).inspect

# super inside an overridden attr reader/writer reaches the parent's attr
class AttrBase
  attr_accessor :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end
end

class Loud < AttrBase
  def name = super.upcase

  def name=(v)
    super(v + "!")
  end
end

l = Loud.new("ann")
puts l.name
l.name = "bob"
puts l.name

# include?/== on collections use a user-defined typed ==
class Vec
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (Vec) -> bool
  def ==(o) = x == o.x
end

#: (Integer) -> Vec
def vec(n) = Vec.new(n)

vs = [vec(1), vec(2)] #: Array[Vec]
puts vs.include?(vec(1)).inspect, vs.include?(vec(3)).inspect
puts (vs == [vec(1), vec(2)]).inspect, (vs == [vec(2), vec(1)]).inspect
h = { a: vec(1) } #: Hash[Symbol, Vec]
puts (h == { a: vec(1) }).inspect

# zsuper from a no-arg initialize lets the parent apply its own default
class Node
  attr_reader :id #: Integer

  #: (?Integer) -> void
  def initialize(id = 1)
    @id = id
  end
end

class Box < Node
  #: () -> void
  def initialize
    super
  end
end

puts Box.new.id.inspect

# Method-name tables (decision 77): public_instance_methods/instance_methods
# list public methods of the class and its ancestors, short of Object.
module PimShared
  def test_shared = 1
end

class PimBase
  def test_base = 1
  def helper = 2

  private

  def test_private = 3
end

class PimChild < PimBase
  include PimShared

  def test_child = 4
  def ==(other) = true
end

p PimChild.public_instance_methods(true).grep(/^test_/).sort
p PimChild.public_instance_methods(false).sort, PimChild.instance_methods(false).sort
p PimChild.method_defined?(:test_base), PimChild.public_method_defined?("test_private"), PimChild.method_defined?(:test_base, false)
pim_k = PimChild #: singleton(PimBase)
p pim_k.public_instance_methods.grep(/^test_/).map(&:to_s).sort, PimShared.instance_methods.sort
