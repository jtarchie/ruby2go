# rbs_inline: enabled
# args: --seed 1
# Class methods, class objects as values, `singleton(C)`, `klass.new`,
# `self.class`, and virtual dispatch of class methods.

require "minitest/autorun"

class Shape
  attr_reader :size #: Integer

  #: (Integer) -> void
  def initialize(size)
    @size = size
  end

  #: () -> String
  def self.kind = "shape"

  #: (Integer) -> bool
  def self.fits?(n) = false

  # `new` on an implicit self dispatches to the receiving class.
  #: (Integer) -> Shape
  def self.build(n) = new(n * 10)

  #: () -> String
  def describe = "#{self.class.kind} #{self.class.name}(#{size})"
end

class Square < Shape
  def self.kind = "square"

  def self.fits?(n) = n.even?
end

class Circle < Shape
  def self.fits?(n) = n.odd?
end

SHAPES = [Square, Circle] #: Array[singleton(Shape)]

#: (Integer) -> singleton(Shape)
def pick(n) = SHAPES.find { |k| k.fits?(n) } || Shape

class ClassMethodsTest < Minitest::Test
  # A class object picked at runtime, then `klass.new`; `describe` reads
  # the class method through `self.class`.
  #: () -> void
  def test_class_objects_as_values
    got = [1, 2, 0].map do |n|
      klass = pick(n)
      shape = klass.new(n)
      [shape.describe, klass.name, klass == Square, klass.to_s]
    end
    assert_equal [
      ["shape Circle(1)", "Circle", false, "Circle"],
      ["square Square(2)", "Square", true, "Square"],
      ["square Square(0)", "Square", true, "Square"],
    ], got
  end

  # Subclasses override class methods; Circle inherits Shape's.
  #: () -> void
  def test_class_method_dispatch
    assert_equal ["shape", "square", "shape"], [Shape.kind, Square.kind, Circle.kind]
  end

  # `build` calls `new` on the receiving class, not on Shape.
  #: () -> void
  def test_new_on_implicit_self
    assert_equal "square Square(40)", Square.build(4).describe
    assert_equal "shape Circle(3)", Circle.new(3).describe
  end

  #: () -> void
  def test_class_names
    assert_equal "Shape", Shape.to_s
    assert_equal "Square", Square.name
  end
end
