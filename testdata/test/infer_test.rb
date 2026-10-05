# rbs_inline: enabled

require "minitest/autorun"

# Parameter types from use (#56, decision 146): no annotation on these
# defs, lambdas or blocks; each takes its types from the calls the
# program makes.

def infer_twice(x) = x * 2
def infer_chain(y) = infer_twice(y) + 1
def infer_label(v, prefix = "n=") = "#{prefix}#{v}"
def infer_maybe(z) = z.nil? ? "none" : z.to_s
def infer_kw(a:, b: 10) = a + b

module InferTests
  class Numerous
    include Enumerable

    def initialize(*list)
      @list = list.empty? ? [2, 5, 3, 6, 1, 4] : list
    end

    def each
      @list.each { |i| yield i }
    end
  end

  class Pairs
    def initialize(names)
      @names = names
    end

    def each_pair
      @names.each_with_index { |n, i| yield i, n }
    end

    def first_or(fallback)
      return fallback unless block_given?
      yield @names.first
      fallback
    end
  end

  class Shape
    def initialize(sides) = (@sides = sides)
    def sides = @sides
  end

  class Square < Shape
    def initialize = super(4)
  end

  class InferTest < Minitest::Test
    def test_method_params
      assert_equal [41, "n=3", "x:4", "none", "5"], [infer_chain(20), infer_label(3), infer_label(4, "x:"), infer_maybe(nil), infer_maybe(5)]
      assert_equal [11, 3], [infer_kw(a: 1), infer_kw(a: 1, b: 2)]
      assert_equal [3, 4], [Shape.new(3).sides, Square.new.sides]
    end

    def test_enumerable_from_each
      assert_equal [[1, 2, 3, 4, 5, 6], [30, 10], 1, 21], [Numerous.new.sort, Numerous.new(3, 1).map { |x| x * 10 }, Numerous.new.min, Numerous.new.sum]
    end

    def test_block_from_yield
      out = [] #: Array[String]
      Pairs.new(%w[a b]).each_pair { |i, n| out << "#{i}#{n}" }
      assert_equal %w[0a 1b], out
      seen = [] #: Array[String?]
      assert_equal [7, 8], [Pairs.new(["z"]).first_or(7), Pairs.new(["q"]).first_or(8) { |s| seen << s }]
      assert_equal ["q"], seen
    end

    def test_lambda_params
      add = ->(a, b) { a + b }
      shout = lambda { |s| s.upcase }
      double = proc { |x| x * 2 }
      assert_equal [3, 7, 11, "HI", [2, 4]], [add.call(1, 2), add.(3, 4), add[5, 6], shout.call("hi"), [1, 2].map(&double)]
    end
  end
end
