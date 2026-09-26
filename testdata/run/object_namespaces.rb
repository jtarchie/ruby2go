# rbs_inline: enabled

# Nested modules/classes, `A::B` paths, lexical then ancestor then top-level constant lookup, constant init order.

#: (String) -> String
def trace(s)
  puts "init #{s}"
  s
end

puts "start"

NAME = "top"
LIMIT = 10
FIRST = trace("first")

module Outer
  NAME = "outer"
  SECOND = trace("second")
  LIST = [1, 2, 3]
  TABLE = { "a" => 1 }
  EMPTY = [] #: Array[String]

  module Inner
    DEPTH = 2

    class Leaf
      #: () -> String
      def where = "#{NAME} #{DEPTH} #{LIMIT} #{SECOND}"

      #: () -> String
      def top = ::NAME
    end
  end

  class Base
    SCALE = 3

    #: () -> Integer
    def scale = SCALE
  end

  class Derived < Base
    #: () -> Integer
    def doubled = SCALE * 2

    #: () -> Integer
    def list_sum = LIST.reduce(0) { |a, b| a + b }
  end
end

puts "middle"

class Outer::Compact
  #: () -> String
  def name_seen = NAME

  #: () -> String
  def qualified = Outer::NAME
end

class Other < Outer::Base
  #: () -> Integer
  def inherited_scale = SCALE + 1
end

module Outer
  THIRD = trace("third")

  class Vec
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
    end

    #: (Vec) -> Vec
    def plus(o) = Vec.new(v + o.v)
  end

  module Factory
    #: () -> Array[Vec]
    def self.make = [Vec.new(1), Vec.new(-2)]

    #: (singleton(Base)) -> Base
    def self.build(k) = k.new
  end

  ORIGIN = Inner::Leaf.new #: Inner::Leaf

  class Compact
    #: () -> String
    def reopened = NAME
  end
end

puts Outer::Inner::Leaf.new.where, Outer::Inner::Leaf.new.top
puts Outer::Derived.new.scale.inspect, Outer::Derived.new.doubled.inspect, Outer::Derived.new.list_sum.inspect
puts Outer::Compact.new.name_seen, Outer::Compact.new.qualified, Outer::Compact.new.reopened
puts Other.new.inherited_scale.inspect, Other.new.scale.inspect
puts Outer::NAME, Outer::Inner::DEPTH.inspect, Outer::Base::SCALE.inspect, Outer::LIST.inspect, Outer::TABLE.inspect
puts Outer::EMPTY.inspect, Outer::ORIGIN.where, ::LIMIT.inspect, FIRST, Outer::THIRD
puts Outer.name, Outer::Inner.name, Outer::Inner::Leaf.name, Outer::Compact.name, Other.name
puts Outer::Inner::Leaf, Outer::Inner::Leaf.inspect, Outer::Derived.new.class, Outer::Derived.new.class.name.inspect
puts Outer.class, Outer::Inner.class, Outer::Base.class

Outer::LIST << 4
puts Outer::LIST.inspect, Outer::Derived.new.list_sum.inspect
puts Outer::Factory.make.map(&:v).inspect, Outer::Factory.make.fetch(0).plus(Outer::Factory.make.fetch(1)).v.inspect
puts Outer::Factory.build(Outer::Derived).scale.inspect, Outer::Factory.build(Outer::Derived).class

# Classes nested in a class and inheriting from it; a nested constant shadowing the outer one;
# relative paths (Config::PORT) resolved from a sibling module's lexical scope.
class Expr
  PRECEDENCE = 0

  #: () -> String
  def show = "?"

  class Num < Expr
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
    end

    def show = v.to_s
  end

  class Add < Expr
    PRECEDENCE = 1

    #: (Expr, Expr) -> void
    def initialize(l, r)
      @l = l
      @r = r
    end

    def show = "(#{@l.show} + #{@r.show})"

    #: () -> Integer
    def prec = PRECEDENCE

    #: (Integer, Integer) -> Add
    def self.of(a, b) = new(Num.new(a), Num.new(b))
  end

  #: () -> Integer
  def prec = PRECEDENCE
end

module App
  module Config
    PORT = 80
  end

  module Server
    #: () -> Integer
    def self.port = Config::PORT + 1

    class Handler
      #: () -> String
      def where = "#{Config::PORT}/#{Server.port}"
    end
  end
end

ex = Expr::Add.new(Expr::Num.new(1), Expr::Add.of(2, 3))
puts ex.show, ex.prec.inspect, Expr::Num.new(4).prec.inspect, Expr.new.prec.inspect
puts Expr::Add.of(5, 6).show, Expr::Add.name, Expr::Num.new(1).class, Expr::Add::PRECEDENCE.inspect, Expr::PRECEDENCE.inspect
puts App::Server.port.inspect, App::Server::Handler.new.where, App::Server::Handler.name, App::Config::PORT.inspect
exprs = [Expr::Num.new(7), ex] #: Array[Expr]
puts exprs.map(&:show).inspect, exprs.map { |x| x.is_a?(Expr::Add) }.inspect, exprs.map(&:class).inspect

# RBS names in annotations resolve like constants: through the superclass (Item, LIMIT from Holder)
# and lexically outward (Vec is Outer::Vec inside Outer::Inner).
class Holder
  class Item
    #: () -> String
    def hi = "item"
  end

  LIMIT = 3
end

class Taker < Holder
  #: () -> Item
  def make = Item.new

  #: (Array[Item]) -> Integer
  def count(xs) = xs.size + LIMIT
end

module Outer
  module Inner
    #: (Integer) -> Vec
    def self.vec(n) = Vec.new(n)
  end
end

tk = Taker.new
puts tk.make.hi, tk.count([tk.make, Holder::Item.new]).inspect, Outer::Inner.vec(4).plus(Outer::Inner.vec(1)).v.inspect
