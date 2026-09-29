# rbs_inline: enabled
# args: --seed 1
# Struct.new and Data.define classes. Member types come from a `#: T` after
# each member (rbs-inline's form), or from a `#: [A, B]` after the whole
# definition. Nilable Struct members may be left out of `new`; every Data
# member is required, and Data values are immutable (`with` copies).

require "minitest/autorun"

Point = Struct.new(
  :x, #: Integer
  :y  #: Integer
)

Resource = Struct.new(:id, :label) do
  #: () -> String
  def describe = "resource #{id.inspect} (#{label})"

  #: (String?) -> Resource
  def self.build(id) = new(id, "built")
end #: [String?, String?]

class Labeled < Resource
  def describe = "labeled: #{super}"
end

Coord = Data.define(
  :lat, #: Float
  :lng  #: Float
) do
  #: () -> String
  def label = "#{lat},#{lng}"
end

class StructTest < Minitest::Test
  #: () -> void
  def test_accessors_and_equality
    point = Point.new(3, 4)
    point.x = 5
    assert_equal [5, 4], [point.x, point.y]
    assert_equal "#<struct Point x=5, y=4>", point.inspect
    assert_equal "#<struct Point x=5, y=4>", point.to_s
    # Structs compare by value.
    assert_equal true, point == Point.new(5, 4)
    assert_equal false, point == Point.new(0, 0)
  end

  #: () -> void
  def test_members_and_conversions
    point = Point.new(5, 4)
    assert_equal [5, 4], point.to_a
    assert_equal [:x, :y], point.members
    assert_equal [:x, :y], Point.members
    assert_equal "#<struct Point x=1, y=7>", Point.new(y: 7, x: 1).inspect
    assert_equal "{x: 5, y: 4}", point.to_h.inspect
  end

  # Methods from the `do` block, a class method, and a subclass calling `super`.
  #: () -> void
  def test_block_methods_and_subclass
    assert_equal "resource \"1\" (one)", Resource.new("1", "one").describe
    assert_equal "resource nil (built)", Resource.build(nil).describe
    assert_equal "labeled: resource \"2\" (two)", Labeled.new("2", "two").describe
  end

  # Nilable members may be omitted, positionally or by keyword.
  #: () -> void
  def test_optional_members
    assert_equal '#<struct Resource id="1", label="one">', Resource.new("1", "one").inspect
    assert_equal '#<struct Labeled id=nil, label="x">', Labeled.new(nil, "x").inspect
    assert_equal "#<struct Resource id=nil, label=nil>", Resource.new.inspect
    assert_nil Resource.new(nil).label
    assert_equal '#<struct Resource id=nil, label="kw">', Resource.new(label: "kw").inspect
  end
end

class DataTest < Minitest::Test
  #: () -> void
  def test_with_copies
    here = Coord.new(lat: 1.5, lng: 2.0)
    moved = here.with(lng: 3.25)
    assert_equal "#<data Coord lat=1.5, lng=2.0>", here.inspect
    assert_equal "#<data Coord lat=1.5, lng=3.25>", moved.inspect
    assert_equal ["1.5,2.0", "1.5,3.25"], [here.label, moved.label]
  end

  #: () -> void
  def test_value_equality_and_members
    here = Coord.new(lat: 1.5, lng: 2.0)
    assert_equal true, here == Coord.new(1.5, 2.0)
    assert_equal false, here == here.with(lng: 3.25)
    assert_equal "{lat: 1.5, lng: 3.25}", here.with(lng: 3.25).to_h.inspect
    assert_equal [:lat, :lng], Coord.members
  end
end
