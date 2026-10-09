# typed: true

require "minitest/autorun"
require "sorbet-runtime"

# Sorbet `sig {}`, `T.let`, `T.cast` and `T.must` read as types (#95, decision 196).
module SorbetTests
  class Shape
    extend T::Sig
    extend T::Helpers
    abstract!

    sig { abstract.returns(Float) }
    def area = raise(NotImplementedError)

    sig { returns(String) }
    def label = "#{self.class.name&.split("::")&.last}(#{area.round(1)})"
  end

  class Circle < Shape
    extend T::Sig

    sig { params(r: Float).void }
    def initialize(r)
      @r = r
    end

    sig { override.returns(Float) }
    def area = 3.14159 * @r * @r
  end

  class Rect < Shape
    extend T::Sig

    sig { returns(Float) }
    attr_accessor :w

    sig { params(h: Float).returns(Float) }
    attr_writer :h

    sig { params(w: Float, h: Float).void }
    def initialize(w, h)
      @w = w
      @h = h
    end

    sig { override.returns(Float) }
    def area = @w * @h

    sig { params(factor: Float).returns(T.attached_class) }
    def self.square(factor) = new(factor, factor)
  end

  class Args
    extend T::Sig

    sig do
      params(a: Integer, b: Integer, rest: String, key: Symbol, opt: T.nilable(String), more: Integer)
        .returns(String)
    end
    def self.all(a, b = 2, *rest, key:, opt: nil, **more)
      "#{a} #{b} #{rest.join(",")} #{key} #{opt.inspect} #{more.keys.sort.join(",")}"
    end

    sig { params(x: T.any(Integer, String)).returns(String) }
    def self.either(x)
      case x
      when Integer then "int #{x}"
      else "str #{x}"
      end
    end

    sig { params(pair: [String, Integer]).returns({ name: String, n: Integer }) }
    def self.shape(pair) = { name: pair[0], n: pair[1] }

    sig { params(blk: T.nilable(T.proc.params(x: Integer).returns(Integer))).returns(Integer) }
    def self.maybe_yield(&blk)
      blk ? blk.call(20) : -1
    end

    sig { params(flag: ::T::Boolean, blk: T.nilable(Proc)).returns(String) }
    def self.loose_block(flag, &blk)
      blk ? "block #{flag}" : "none #{flag}"
    end

    sig { params(set: T::Set[Integer]).returns(Integer) }
    def self.set_size(set) = set.size

    sig { params(klass: T.class_of(Shape)).returns(String) }
    def self.class_name(klass) = klass.name.to_s

    class << self
      extend T::Sig

      sig { params(n: Integer).returns(Integer) }
      def twice(n) = n * 2
    end

    sig { params(n: Integer).returns(Integer) }
    private_class_method def self.hidden(n) = n + 1

    sig { params(n: Integer).returns(Integer) }
    def self.call_hidden(n) = hidden(n)
  end

  class SorbetTest < Minitest::Test
    def test_abstract_and_final_classes
      shapes = T.let([Circle.new(1.0), Rect.new(2.0, 3.0), Rect.square(2.0)], T::Array[Shape])
      assert_equal ["Circle(3.1)", "Rect(6.0)", "Rect(4.0)"], shapes.map(&:label)
    end

    def test_attr_sigs
      r = Rect.new(1.0, 1.0)
      r.w = 4.0
      r.h = 0.5
      assert_in_delta 2.0, r.area
      assert_in_delta 4.0, r.w
    end

    def test_parameter_kinds
      assert_equal "1 2  k nil ", Args.all(1, key: :k)
      assert_equal "1 5 x,y k \"o\" p,q", Args.all(1, 5, "x", "y", key: :k, opt: "o", q: 1, p: 2)
    end

    def test_union_tuple_shape
      assert_equal "int 3", Args.either(3)
      assert_equal "str x", Args.either("x")
      assert_equal({ name: "a", n: 1 }, Args.shape(["a", 1]))
    end

    def test_nilable_block
      assert_equal 21, Args.maybe_yield { |x| x + 1 }
      assert_equal(-1, Args.maybe_yield)
      assert_equal "block true", Args.loose_block(true) { 1 }
      assert_equal "none false", Args.loose_block(false)
    end

    def test_generic_and_class_types
      assert_equal 2, Args.set_size(Set[1, 2, 2])
      assert_equal "SorbetTests::Circle", Args.class_name(Circle)
      assert_equal 8, Args.twice(4)
      assert_equal 5, Args.call_hidden(4)
    end

    def test_let_cast_must
      n = T.let(nil, T.nilable(Integer))
      assert_nil n
      xs = %w[a bcd].map { |x| x.length + T.let(1, Integer) }
      assert_equal [2, 4], xs
      untyped = T.unsafe("cast me")
      assert_equal "CAST ME", T.cast(untyped, String).upcase
      assert_equal 3, T.must([1, 2, 3].max)
      begin
        T.must([].first)
        flunk "no TypeError"
      rescue TypeError => e
        assert_equal "Passed `nil` into T.must", e.message
      end
    end
  end
end
