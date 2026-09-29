# rbs_inline: enabled
# args: --seed 1
# Nested modules and classes, `A::B` paths, constants, lexical lookup.

require "minitest/autorun"

module Geometry
  VERSION = "1.2.0" #: String

  module Units
    SCALE = 100
  end

  class Error < StandardError; end

  class Point
    attr_reader :x #: Integer
    attr_reader :y #: Integer

    #: (Integer, Integer) -> void
    def initialize(x, y)
      @x = x
      @y = y
    end

    #: () -> String
    def to_s = "(#{x}, #{y})"

    # `Units` resolves lexically through Geometry.
    #: () -> Integer
    def scaled_x = x * Units::SCALE
  end

  # `Point` resolves lexically too.
  class Labeled < Point
    attr_reader :label #: String

    #: (Integer, Integer, String) -> void
    def initialize(x, y, label)
      super(x, y)
      @label = label
    end

    def to_s = "#{label}@#{super}"
  end
end

# Compact form: only Geometry::Circle is in the lexical scope here.
class Geometry::Circle < Geometry::Point
  attr_reader :r #: Integer

  #: (Integer, Integer, Integer) -> void
  def initialize(x, y, r)
    super(x, y)
    @r = r
  end

  def to_s = "circle #{super} r=#{r}"
end

module Geometry
  ORIGIN = Point.new(0, 0) #: Point
end

LIMIT = 3

class NamespacesTest < Minitest::Test
  #: () -> void
  def test_nested_class_and_lexical_lookup
    labeled = Geometry::Labeled.new(1, 2, "a")
    assert_equal "a@(1, 2)", labeled.to_s
    assert_equal 100, labeled.scaled_x
  end

  #: () -> void
  def test_constant_paths
    assert_equal "1.2.0", Geometry::VERSION
    assert_equal 100, Geometry::Units::SCALE
    assert_equal "(0, 0)", Geometry::ORIGIN.to_s
    assert_equal 4, LIMIT + 1
  end

  # A class defined in the compact `Geometry::Circle` form.
  #: () -> void
  def test_compact_class_definition
    assert_equal "circle (0, 0) r=5", Geometry::Circle.new(0, 0, 5).to_s
  end

  # With no message, an exception's message is its class name.
  #: () -> void
  def test_namespaced_exception
    e = assert_raises(Geometry::Error) { raise Geometry::Error }
    assert_equal "Geometry::Error", e.message
  end
end
