# rbs_inline: enabled

require "minitest/autorun"

# Generic struct classes (#104, decision 308): one Go struct per instantiation, one class object.

module GenericClassTests
  # @rbs generic T
  class Box
    #: (T) -> void
    def initialize(v)
      @v = v
    end

    #: () -> T
    def get = @v

    #: (T) -> void
    def set(v)
      @v = v
    end

    #: () -> Array[T]
    def twice = [@v, @v]
  end

  # @rbs generic E
  class Stack
    #: (Array[E]) -> void
    def initialize(items)
      @items = items
    end

    #: (E) -> self
    def push(x)
      @items << x
      self
    end

    #: () -> E?
    def pop = @items.pop

    #: () -> Integer
    def size = @items.size

    #: () -> Box[E]?
    def top_box
      x = @items.last
      return nil if x.nil?
      Box.new(x)
    end
  end

  # @rbs generic E
  class Bag
    include Enumerable #[E]

    #: (Array[E]) -> void
    def initialize(items)
      @items = items
    end

    #: () { (E) -> void } -> void
    def each(&)
      @items.each { |x| yield x }
    end
  end

  # @rbs generic K
  # @rbs generic V
  class Pair
    attr_reader :k #: K
    attr_reader :v #: V

    #: (K, V) -> void
    def initialize(k, v)
      @k = k
      @v = v
    end

    #: () -> Pair[V, K]
    def swap = Pair.new(@v, @k)
  end

  class Point
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end
  end

  class GenericClassTest < Minitest::Test
    def test_two_instantiations
      a = Box.new(1)
      s = Box.new("a")
      assert_equal 3, a.get + 2
      assert_equal "ab", s.get + "b"
      a.set(5)
      assert_equal [5, 5], a.twice
    end

    def test_annotated_instance
      b = Box.new(nil) #: Box[Integer?]
      assert_nil b.get
      b.set(3)
      assert_equal 3, b.get
    end

    def test_class_and_is_a
      a = Box.new(1)
      s = Box.new("a")
      assert_equal Box, a.class
      assert_same a.class, s.class
      assert a.is_a?(Box)
      assert_kind_of Box, s
      assert_equal "GenericClassTests::Box", Box.name
    end

    def test_inspect_and_ivars
      a = Box.new(7)
      assert_match(/\A#<GenericClassTests::Box:0x\h+ @v=7>\z/, a.inspect)
      assert_equal 7, a.instance_variable_get(:@v)
      assert_equal [:@v], a.instance_variables
    end

    def test_stack_instances
      ints = Stack.new([1, 2])
      strs = Stack.new(["x"])
      ints.push(3).push(4)
      assert_equal 4, ints.size
      assert_equal 4, ints.pop
      assert_equal "x", strs.pop
      assert_nil strs.pop
      assert_equal 3, ints.top_box&.get
    end

    def test_class_argument
      pts = Stack.new([Point.new(1)])
      pts.push(Point.new(2))
      assert_equal 2, pts.pop&.x
    end

    def test_enumerable_instances
      ints = Bag.new([3, 1, 2])
      strs = Bag.new(%w[b a])
      assert_equal [1, 2, 3], ints.sort
      assert_equal [6, 2, 4], ints.map { |x| x * 2 }
      assert_equal "a", strs.min
      assert strs.include?("a")
    end

    def test_two_parameters
      pr = Pair.new(1, "one")
      assert_equal "one", pr.swap.k
      assert_equal 1, pr.swap.v
      assert_equal "one", pr.swap.swap.v
      assert_instance_of Pair, pr.swap
      refute_instance_of Box, pr
    end

    def test_untyped_receiver
      items = [Box.new(1), Box.new("a")] #: Array[untyped]
      assert_equal [1, "a"], items.map(&:get)
      assert_equal [true, true], items.map { |x| x.is_a?(Box) }
      assert_equal 1, items[0].send(:get)
      assert_equal [Box, Box], items.map(&:class)
    end

    def test_case_when
      v = Box.new(1) #: untyped
      kind = case v
             when Stack then :stack
             when Box then :box
             else :other
             end
      assert_equal :box, kind
      assert Box === v
      refute Stack === v
    end
  end
end
