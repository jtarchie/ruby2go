# rbs_inline: enabled

LIMIT = 3

module Greets
  #: (?String) -> String
  def hello(who = default_who) = "hello #{who}"

  #: () -> String
  def default_who = "module"
end

class Base
  include Greets

  #: () -> void
  def initialize
    @n = 10
  end

  #: (?Integer, ?Integer) -> Integer
  def calc(a = @n, b = a * 2) = a + b

  #: (Integer, ?Integer, *Integer) -> Integer
  def sum(first, second = first + 1, *rest) = first + second + rest.size

  #: (?Integer) { (Integer) -> void } -> void
  def times_up(n = LIMIT)
    n.times { |i| yield i }
  end
end

class Kid < Base
  #: (?Integer, ?Integer) -> Integer
  def calc(a = 1, b = 2) = super() * 100 + a + b

  #: () -> String
  def default_who = "kid"
end

class Box
  #: (Integer, ?Integer) -> void
  def initialize(w, h = w + 1)
    @w = w
    @h = h
  end

  #: () -> Integer
  def area = @w * @h
end

class Shape
  #: (?String) -> String
  def self.make(kind = default_kind) = "made #{kind}"

  #: () -> String
  def self.default_kind = "shape"
end

class Circle < Shape
  #: () -> String
  def self.default_kind = "circle"
end

class Log
  #: () -> void
  def initialize
    @lines = [] #: Array[String]
  end

  #: (String) -> String
  def note(s)
    @lines << s
    s
  end

  # the default is never read, but runs for its side effect
  #: (String, ?String) -> String
  def f(a, b = note("side")) = a

  #: () -> Integer
  def count = @lines.size
end

#: (Integer, ?Integer) -> Integer
def top(a, b = a + LIMIT) = a * b

#: (Base) -> Integer
def via_base(b) = b.calc

b = Base.new
puts b.calc, b.calc(1), b.calc(1, 1)
puts b.sum(1), b.sum(1, 5), b.sum(1, 5, 7, 8)
b.times_up { |i| print i }
b.times_up(2) { |i| print i }
puts
puts via_base(Kid.new), Kid.new.calc(5)
puts b.hello, Kid.new.hello, b.hello("you")
puts Box.new(3).area, Box.new(3, 3).area
puts Shape.make, Circle.make, Circle.make("x")
puts top(2), top(2, 2)
l = Log.new
puts l.f("x"), l.f("y", "z"), l.count
