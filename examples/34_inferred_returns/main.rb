# rbs_inline: enabled
# args: --seed 1

require "stringio"
require "minitest/autorun"

class Rect
  attr_reader :w #: Integer
  attr_reader :h #: Integer

  #: (Integer, Integer) -> void
  def initialize(w, h)
    @w = w
    @h = h
  end

  def area = w * h

  def label
    return "square" if w == h
    "rect #{area}"
  end

  def big? = area > 10

  # @rbs by: Integer
  def grow(by)
    Rect.new(w + by, h + by)
  end

  # An early `return nil` joins with the Integer below: Integer?.
  def half_width
    return nil if w.odd?
    w / 2
  end

  # @rbs io: StringIO
  def shout(io)
    io.puts label.upcase
  end
end

class Square < Rect
  def label = "sq"
end

# @rbs xs: Array[Integer]
# @rbs scale: Integer
def widest(xs, scale = 2)
  xs.map { |x| x * scale }.max
end

# @rbs name: String
# @rbs return: String
def greet(name) = "hi #{name}"

def origin = Rect.new(0, 0)

class InferredReturnsTest < Minitest::Test
  def test_methods_without_annotations_infer_their_return_type
    r = Rect.new(3, 4)
    assert_equal 12, r.area
    assert_equal "rect 12", r.label
    assert_equal true, r.big?
    assert_equal 20, r.grow(1).area
  end

  def test_early_return_nil_makes_the_result_optional
    assert_nil Rect.new(3, 4).half_width
    assert_equal 1, Rect.new(2, 2).half_width
    assert_equal "square", Rect.new(2, 2).label
  end

  def test_a_void_method_is_inferred_too
    io = StringIO.new
    Rect.new(3, 4).shout(io)
    assert_equal "RECT 12\n", io.string
  end

  def test_an_unannotated_override_keeps_the_parent_type
    s = Square.new(1, 1) #: Rect
    assert_equal "sq", s.label
  end

  def test_per_parameter_rbs_annotations
    assert_equal 6, widest([1, 2, 3])
    assert_nil widest([], 5)
    assert_equal "hi bo", greet("bo")
    assert_equal 0, origin.area
  end
end
