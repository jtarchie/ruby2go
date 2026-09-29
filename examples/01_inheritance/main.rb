# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class Shape
  #: () -> String
  def name = "Shape"

  #: () -> Float
  def area = raise(NotImplementedError)

  #: () -> String
  def describe = "#{name}: #{area}"
end

class Rect < Shape
  attr_reader :w #: Float
  attr_reader :h #: Float

  #: (Float, Float) -> void
  def initialize(w, h)
    @w = w
    @h = h
  end

  def name = "Rect"
  def area = w * h
end

class Square < Rect
  #: (Float) -> void
  def initialize(side) = super(side, side)

  def name = "Square"
end

class InheritanceTest < Minitest::Test
  # `describe` lives on Shape but calls the subclass's `name` and `area`.
  #: () -> void
  def test_describe_dispatches_to_subclass
    shapes = [Rect.new(2.0, 3.0), Square.new(2.0)] #: Array[Shape]
    assert_equal ["Rect: 6.0", "Square: 4.0"], shapes.map(&:describe)
  end

  # Square#initialize forwards to Rect's through `super`.
  #: () -> void
  def test_super_initialize
    sq = Square.new(2.0)
    assert_equal 2.0, sq.w
    assert_equal 2.0, sq.h
  end

  #: () -> void
  def test_abstract_area_raises
    assert_raises(NotImplementedError) { Shape.new.describe }
  end
end
