# typed: true

require "minitest/autorun"
require "sorbet-runtime"

# Sorbet `sig {}`, `T.let`, `T.cast` and `T.must` read as types (#95, decision 196).
module SorbetTests
  class BindReq
    extend T::Sig

    # @rbs self.@check: (^() [self: BindReq] -> untyped)?
    @check = nil

    sig { params(block: T.proc.bind(BindReq).returns(T.untyped)).void }
    def self.satisfy(&block)
      @check = block
    end

    sig { returns(T.nilable(T.proc.bind(BindReq).returns(T.untyped))) }
    def self.check = @check

    sig { returns(T.nilable(T.proc.bind(BindReq).returns(T.untyped))) }
    def self.none = nil

    # `&p` of a nil Proc passes no block (Homebrew's `Homebrew::Service.new(self, &self.class.service)`)
    sig { params(block: T.nilable(T.proc.bind(BindReq).returns(T.untyped))).returns(T::Boolean) }
    def self.given?(&block) = !block.nil?

    sig { returns(T.untyped) }
    def satisfied?
      c = BindReq.check
      return true unless c

      instance_eval(&c)
    end
  end

  class BindMac < BindReq
    extend T::Sig

    sig { params(v: Integer).void }
    def initialize(v) = @v = v

    sig { returns(T::Boolean) }
    def new_enough? = @v > 10
  end

  BindReq.satisfy do
    T.bind(self, BindMac)
    new_enough? && @v.positive? # its ivars are BindMac's (Homebrew's ArchRequirement `@arch`)
  end

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

  class Field < T::Enum
    enums do
      Name = new
      Desc = new("description")
    end

    def label = serialize.upcase
  end

  class Args
    extend T::Sig

    # a frozen literal of lambdas cast as a whole (Homebrew's SearchCmd::PACKAGE_MANAGERS)
    URLS = T.let({
      a: ->(q) { "a=#{q}" },
      b: lambda { |q|
        "b=#{q}"
      }
    }.freeze, T::Hash[Symbol, T.proc.params(query: String).returns(String)])

    sig { params(f: Field).returns(String) }
    def self.field_kind(f)
      case f
      when Field::Name then "name"
      when Field::Desc then "desc"
      else "other"
      end
    end

    YieldType = T.type_alias { T.nilable(T.proc.params(x: Integer).returns(Integer)) }

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

    # a class object of a type parameter (Homebrew's Contributions#cached_github_request)
    sig do
      type_parameters(:U).params(type: T::Class[T.type_parameter(:U)], blk: T.proc.returns(T.type_parameter(:U)))
                         .returns(T.nilable(T.type_parameter(:U)))
    end
    def self.typed_yield(type, &blk)
      v = blk.call
      v.is_a?(type) ? v : nil
    end

    # an untyped block may be left out (Homebrew's LazyObject#__getobj__(&_blk))
    sig { params(blk: T.untyped).returns(String) }
    def self.maybe_untyped_block(&blk) = blk ? "block" : "none"

    # a block typed by an alias (Homebrew's Search `block: SearchBlockType`)
    sig { params(blk: YieldType).returns(Integer) }
    def self.alias_yield(&blk)
      blk ? blk.call(30) : -1
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

  # `<=>` typed for any other object, nil when it cannot compare: Comparable still answers through it
  class Tok
    extend T::Sig
    include Comparable

    sig { returns(Integer) }
    attr_reader :v

    sig { params(v: Integer).void }
    def initialize(v)
      @v = v
    end

    sig { params(other: BasicObject).returns(T.nilable(Integer)) }
    def <=>(other)
      case other
      when Tok then v <=> other.v
      end
    end
  end

  class RevTok < Tok
    extend T::Sig

    sig { params(other: BasicObject).returns(T.nilable(Integer)) }
    def <=>(other)
      case other
      when RevTok then other.v <=> v
      else super
      end
    end
  end

  # Sorbet's way of typing Enumerable: a fixed `Elem` type member
  class Bag
    extend T::Sig
    extend T::Generic
    include Enumerable

    Elem = type_member(:out) { { fixed: String } }

    sig { params(xs: T::Array[String]).void }
    def initialize(xs)
      @xs = xs
    end

    sig { override.params(block: T.proc.params(x: String).void).returns(T.self_type) }
    def each(&block)
      @xs.each(&block)
      self
    end
  end

  class Registry
    class << self
      extend T::Generic
      include Enumerable

      Elem = type_member(:out) { { fixed: Integer } }
    end

    #: () { (Integer) -> void } -> void
    def self.each(&) = [3, 1, 2].each(&)
  end

  class SorbetTest < Minitest::Test
    extend T::Sig

    sig { params(x: T.all(Comparable, Integer)).returns(Integer) }
    def sorbet_twice(x) = x * 2

    def test_nilable_spaceship
      a = Tok.new(1)
      b = Tok.new(2)
      assert_operator a, :<, b
      assert_equal 2, [b, a].max.v
      assert a.between?(a, b)
      x = RevTok.new(1)
      y = RevTok.new(2)
      assert_operator y, :<, x
    end

    def test_elem_type_member
      assert_equal %w[a b], Bag.new(%w[b a]).sort
      assert_equal [1, 2], Bag.new(%w[x yy]).map(&:size)
      assert_equal [1, 2, 3], Registry.sort
      assert Registry.include?(2)
    end

    def test_intersection
      assert_equal 4, sorbet_twice(2)
    end

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

    def test_frozen_lambda_hash
      assert_equal ["b=x", [:a, :b], true], [Args::URLS.fetch(:b).call("x"), Args::URLS.keys, Args::URLS.frozen?]
    end

    def test_enum
      assert_equal "desc", Args.field_kind(Field::Desc)
      assert_equal "name", Args.field_kind(Field::Name)
      assert_equal ["name", "description"], [Field::Name.serialize, Field::Desc.serialize]
      assert_equal "#<SorbetTests::Field::Desc>", Field::Desc.inspect
      assert_equal "NAME", Field::Name.label
    end

    def test_nilable_block
      assert_equal 21, Args.maybe_yield { |x| x + 1 }
      assert_equal(-1, Args.maybe_yield)
      assert_equal 31, Args.alias_yield { |x| x + 1 }
      assert_equal 3, Args.typed_yield(Integer) { 3 }
      assert_equal ["none", "block"], [Args.maybe_untyped_block, Args.maybe_untyped_block { 1 }]
      assert_equal(-1, Args.alias_yield)
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

    # `T.bind(self, Sub)` in a block bound to Base narrows self (Homebrew's MacOSRequirement satisfy block)
    def test_bind_narrows_self
      assert_equal [true, false], [BindMac.new(11).satisfied?, BindMac.new(3).satisfied?]
      assert_equal [true, false], [BindReq.given?(&BindReq.check), BindReq.given?(&BindReq.none)]
    end

    # T.cast to a tuple, from untyped and from a wider tuple; a multiple assignment of one (Homebrew's
    # `named_formulae, named_casks = T.cast(args.named.to_formulae_to_casks, [...])`).
    def test_cast_tuple
      wide = [[1, 2], [:d]] #: [Array[Integer | String], Array[Symbol]]
      a, b = T.cast(wide, [T::Array[Integer], T::Array[Symbol]])
      assert_equal [3, [:d]], [a.sum, b]
      mixed = [1, :x, 2] #: Array[Integer | Symbol]
      ints, syms2 = T.cast(mixed.partition { |x| x.is_a?(Integer) }, [T::Array[Integer], T::Array[Symbol]])
      assert_equal [3, [:x]], [ints.sum, syms2]
      pair = T.unsafe([[4], [:e]])
      c, d = T.cast(pair, [T::Array[Integer], T::Array[Symbol]])
      assert_equal [4, [:e]], [c.sum, d]
    end
  end
end
