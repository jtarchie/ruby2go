# rbs_inline: enabled

require "minitest/autorun"

# Enumerator, external iteration, Enumerator::Lazy and ArithmeticSequence (decision 140), against MRI.
module EnumeratorTests
  class Basket
    include Enumerable #[String]

    #: (*String) -> void
    def initialize(*items)
      @items = items
    end

    #: () { (String) -> void } -> void
    def each
      @items.each { |x| yield x }
    end
  end

  class Drained
    include Enumerable #[Integer]

    #: (Array[Integer]) -> void
    def initialize(xs)
      @xs = xs
    end

    #: () { (Integer) -> void } -> void
    def each
      e = @xs.each
      loop { yield e.next }
    end
  end

  class ExternalTest < Minitest::Test
    def test_next_peek_rewind
      e = [10, 20].each
      assert_equal 10, e.next
      assert_equal 20, e.peek
      assert_equal 20, e.next
      err = assert_raises(StopIteration) { e.next }
      assert_equal [10, 20], err.result
      assert_raises(StopIteration) { e.peek }
      e.rewind
      assert_equal 10, e.next
    end

    def test_blockless_forms
      t = 3.times
      assert_equal [0, 1, 2], [t.next, t.next, t.next]
      assert_equal 3, t.size
      c = "héy".each_char
      assert_equal ["h", "é"], [c.next, c.next]
      assert_equal 3, c.size
      l = "a\nb\n".each_line
      assert_equal "a\n", l.next
      assert_nil l.size
      w = %w[a b].each_with_index
      assert_equal ["a", 0], w.next
      assert_equal ["b", 1], w.next
      s = [1, 2, 3].each_slice(2)
      assert_equal [1, 2], s.next
      assert_equal [[1, 2], [3]], s.to_a
      assert_equal 2, s.size
      k = [1, 2, 3].each_cons(2)
      assert_equal [1, 2], k.next
      assert_equal [2, 3], k.peek
      assert_equal 2, k.size
      m = [4, 5].map
      assert_equal 4, m.next
      assert_equal [8, 10], m.with_index { |x, i| x * 2 + i * 0 }
      u = 1.upto(3)
      assert_equal [1, 2], [u.next, u.next]
      assert_equal 3, u.size
      assert_equal [3, 2, 1], 3.downto(1).to_a
    end

    def test_prelude_iterators_without_a_block
      r = (1..3).each
      assert_equal [1, 2], [r.next, r.next]
      h = { a: 1, b: 2 }.each
      assert_equal [:a, 1], h.next
      assert_equal [[:a, 0]], { a: 1 }.each_with_index.map { |(k, _v), i| [k, i] }
      assert_equal 97, "abc".each_byte.next
      assert_equal [2, 1], [1, 2].reverse_each.to_a
    end

    def test_chains
      assert_equal [[1, 1], [2, 2]], [1, 2].each.with_index(1).to_a
      got = []
      [1, 2].each.with_index(1) { |x, i| got << x * i }
      assert_equal [1, 4], got
      assert_equal %w[a0 b1], %w[a b].each_with_index.map { |x, i| "#{x}#{i}" }
      assert_equal [3, 6], [1, 2].each.with_object([]) { |x, acc| acc << x * 3 }
      assert_equal [3, 3], [1, 2, 3].each_slice(2).map(&:sum)
      assert_equal ["h", "ee"], "he".each_char.with_index.map { |ch, i| ch * (i + 1) }
      assert_equal [[3, 1], ["x"]], [Basket.new("3", "1").each_slice(2).first.map(&:to_i), Basket.new("x").to_a]
      assert_equal [["a", 0], ["b", 1]], Basket.new("a", "b").each_with_index.to_a
    end

    def test_loop_rescues_stop_iteration
      e = [1, 2, 3].each
      seen = []
      result = loop do
        seen << e.next
      end
      assert_equal [1, 2, 3], seen
      assert_equal [1, 2, 3], result
      n = 0
      loop do
        n += 1
        raise StopIteration if n == 3
      end
      assert_equal 3, n
      assert_equal [5, 6], Drained.new([4, 5]).map { |x| x + 1 }
      src = [1, 2, 3].each
      doubled = Enumerator.new do |y|
        loop { y << src.next * 2 }
      end
      assert_equal [2, 4, 6], doubled.to_a
    end

    def test_inspect
      assert_equal "#<Enumerator: [1, 2]:each>", [1, 2].each.inspect
      assert_equal "#<Enumerator: 5:times>", 5.times.inspect
      assert_equal "#<Enumerator: [1, 2, 3]:each_slice(2)>", [1, 2, 3].each_slice(2).inspect
      assert_equal "#<Enumerator: \"ab\":each_char>", "ab".each_char.inspect
      assert_equal "#<Enumerator: 1..3:each>", (1..3).each.inspect
      assert_equal "#<Enumerator: #<Enumerator: [1]:each>:with_index(1)>", [1].each.with_index(1).inspect
    end

    def test_sizes
      assert_nil ("a".."e").each_slice(2).size
      assert_equal 3, (1..5).each_slice(2).size
      assert_equal 1, { a: 1 }.each_with_index.size
      assert_equal 2, ((1..10) % 3).each_slice(2).size
      assert_equal 3, ((1...10) % 3).size
      assert_equal 0, ((10..1) % 3).size
      assert_equal 4, 10.step(1, -3).size
      assert_equal 500_000_000_000, ((1..10**12) % 2).size
    end
  end

  class GeneratorTest < Minitest::Test
    #: () -> Enumerator[Integer]
    def fib
      Enumerator.new do |y|
        a, b = 0, 1
        loop do
          y << a
          a, b = b, a + b
        end
      end
    end

    def test_take_first_next
      f = fib
      assert_equal [0, 1, 1, 2, 3, 5, 8, 13], f.take(8)
      assert_equal 0, f.first
      assert_equal [0, 1, 1], f.first(3)
      assert_equal [0, 1, 1], [f.next, f.next, f.next]
      assert_equal 2, f.peek
      f.rewind
      assert_equal 0, f.next
      assert_nil f.size
      assert_equal [[0, 1], [1, 2]], f.each_slice(2).first(2)
      assert_equal [[0, 1], [1, 2]], f.with_index(1).take(2)
    end

    def test_finite_generator
      squares = Enumerator.new do |y|
        [1, 2, 3].each { |x| y.yield x * x }
      end
      assert_equal [1, 4, 9], squares.to_a
      assert_equal [2, 5, 10], squares.map { |x| x + 1 }
      assert squares.include?(4)
      assert_equal [4], squares.select(&:even?)
      assert_equal 14, squares.sum
      assert_equal [[1, 1], [4, 2], [9, 3]], squares.with_index(1).to_a
      s = squares.each
      assert_equal [1, 4, 9], [s.next, s.next, s.next]
      assert_raises(StopIteration) { s.next }
      w = [7, 8].each.with_index
      2.times { w.next }
      assert_equal [7, 8], assert_raises(StopIteration) { w.next }.result
    end

    def test_yielder_forms
      words = Enumerator.new { |y| %w[a b].each(&y) }
      assert_equal %w[a b], words.to_a
      procs = Enumerator.new { |y| y << 0; y.to_proc.call(5) }
      assert_equal [0, 5], procs.to_a
      calls = Enumerator.new { |y| y << "c" << "d" }
      assert_equal %w[c d], calls.to_a
      sized = Enumerator.new(3) { |y| y << "x" }
      assert_equal 3, sized.size
    end

    def test_generator_stops_where_the_consumer_does
      log = []
      gen = Enumerator.new do |y|
        y << 1
        log << :after_one
        y << 2
        log << :after_two
      end
      assert_equal [1], gen.first(1)
      assert_equal [], log
      assert_equal [1, 2], gen.take(2)
      assert_equal [:after_one], log
    end
  end

  class LazyTest < Minitest::Test
    def test_pipelines
      assert_equal [6, 12, 18], (1..).lazy.map { |x| x * 2 }.select { |x| x % 3 == 0 }.first(3)
      assert_equal [2, 4, 6], (1..20).lazy.reject(&:odd?).take(3).to_a
      assert_equal [4, 5], (1..).lazy.drop(3).drop_while { |x| x < 4 }.first(2)
      assert_equal [1, -1, 2, -2], (1..).lazy.flat_map { |x| [x, -x] }.first(4)
      assert_equal [[1, 10], [2, 20], [3, nil]], (1..3).lazy.zip([10, 20]).to_a
      assert_equal [1, 4, 9], (1..).lazy.with_index(1).map { |x, i| x * i }.first(3)
      assert_equal [[1, 0], [2, 1]], (1..).lazy.each_with_index.first(2)
      assert_equal [1, 2, 3], [1, 1, 2, 3, 3].lazy.uniq.force
      assert_equal [1, 2], [1, nil, 2].lazy.compact.to_a
      assert_equal [4, 8], (1..).lazy.filter_map { |x| x * 2 if x.even? }.first(2)
      assert_equal %w[1 2], (1..).lazy.map(&:to_s).take_while { |s| s.size < 2 }.first(2)
      assert_equal [2, 4, 6], (1..3).lazy.map { |x| x * 2 }.eager.to_a
      assert (1..).lazy.map { |x| x * x }.include?(49)
      assert_equal 20, (1..4).lazy.map { |x| x * 2 }.sum
      assert_equal 6, (1..).lazy.select(&:even?).map { |x| x * 3 }.first
    end

    def test_over_a_generator_and_infinity
      evens = Enumerator.new do |y|
        a, b = 0, 1
        loop do
          y << a
          a, b = b, a + b
        end
      end
      assert_equal [0, 2, 8], evens.lazy.select(&:even?).first(3)
      assert_equal [1, 2, 3], (1..Float::INFINITY).first(3)
      assert_equal [2, 4], (1..Float::INFINITY).lazy.map { |x| x * 2 }.first(2)
    end

    def test_with_index_block_and_each
      seen = []
      out = (1..3).lazy.with_index { |x, i| seen << x * 10 + i }.to_a
      assert_equal [1, 2, 3], out
      assert_equal [10, 21, 32], seen
      got = []
      (1..3).lazy.map { |x| x + 1 }.each { |x| got << x }
      assert_equal [2, 3, 4], got
    end

    def test_laziness
      pulled = []
      firsts = (1..).lazy.map { |x| pulled << x; x }.select(&:odd?).first(2)
      assert_equal [1, 3], firsts
      assert_equal [1, 2, 3], pulled
    end

    def test_inspect
      assert_equal "#<Enumerator::Lazy: 1..3>", (1..3).lazy.inspect
      assert_equal "#<Enumerator::Lazy: #<Enumerator::Lazy: 1..3>:map>", (1..3).lazy.map { |x| x }.inspect
      assert_equal "#<Enumerator::Lazy: #<Enumerator::Lazy: 1..>:take(2)>", (1..).lazy.take(2).inspect
      assert_equal "#<Enumerator::Lazy: #<Enumerator::Lazy: 1..Infinity>:select>", (1..Float::INFINITY).lazy.select(&:even?).inspect
    end
  end

  class ArithmeticSequenceTest < Minitest::Test
    def test_inspect_and_class
      assert_equal "((1..10).%(3))", ((1..10) % 3).inspect
      assert_equal "(1.step(10, 3))", 1.step(10, 3).inspect
      assert_equal "((1...10).step(3))", (1...10).step(3).inspect
      assert_equal "(1.step(10))", 1.step(10).inspect
      assert_equal "(1.0.step(2.0, 0.5))", 1.0.step(2.0, 0.5).inspect
      assert_equal "((1..).step(2))", (1..).step(2).inspect
      assert_equal "#<Enumerator: \"a\"..\"e\":step(2)>", ("a".."e").step(2).inspect
      assert_equal Enumerator::ArithmeticSequence, ((1..10) % 3).class
    end

    def test_values
      s = 1.step(10, 3)
      assert_equal [1, 10, 3], [s.begin, s.end, s.step]
      refute s.exclude_end?
      assert_equal 1, s.first
      assert_equal [1, 4], s.first(2)
      assert_equal 10, s.last
      assert_equal [7, 10], s.last(2)
      assert_equal 4, s.size
      assert_equal [1, 4, 7, 10], s.to_a
      assert_equal 1.step(10, 3), s
      assert_equal 7, (1...10).step(3).last
      assert_equal [10, 7, 4, 1], 10.step(1, -3).to_a
      assert_equal [2, 8, 14, 20], (1..10).step(3).map { |x| x * 2 }
      assert_equal [1, 3, 5], (1..).step(2).first(3)
      assert_equal [1.0, 1.5, 2.0], 1.0.step(2.0, 0.5).to_a
      assert_equal [10, 40], ((1..10) % 3).lazy.map { |x| x * 10 }.first(2)
      assert_equal [[1, 4], [7, 10]], 1.step(10, 3).each_slice(2).to_a
      assert_equal 22, (1..10).step(3).sum
      assert ((1..10) % 3).include?(4)
      assert_equal %w[a c e], ("a".."e").step(2).to_a
      assert_equal [], (5..1).step(2).to_a
      assert_nil (5..1).step(2).last
      e = (1..3).step(1)
      assert_equal [1, 2], [e.next, e.next]
    end

    def test_errors
      assert_raises(ArgumentError) { (1..10).step(0) }
      err = assert_raises(RangeError) { (1..).step(3).last }
      assert_equal "cannot get the last element of endless arithmetic sequence", err.message
    end
  end
  # #56: min(n)/max(n) on an endless range end, as MRI's do
  class EnumeratorEndlessMinMaxTest < Minitest::Test
    def test_endless_min_max_n
      assert_equal [[1, 2], [], [1, 2], [5, 4]], [(1..).min(2), (1..).min(0), (1..5).min(2), (1..5).max(2)]
      e = assert_raises(RangeError) { (1..).max(2) }
      assert_equal "cannot get the maximum of endless range", e.message
      e = assert_raises(RangeError) { (..1).min(2) }
      assert_equal "cannot get the minimum of beginless range", e.message
      e = assert_raises(ArgumentError) { (1..).min(-1) }
      assert_equal "negative array size (or size too big)", e.message
    end

    # #75: a beginless Integer range counts down from its end
    def test_beginless_max_n
      assert_equal [[5, 4], [4, 3], [], [5, 4, 3]], [(..5).max(2), (...5).max(2), (..5).max(0), (..5).reverse_each.first(3)]
      e = assert_raises(ArgumentError) { (..5).max(-1) }
      assert_equal "negative array size (or size too big)", e.message
      e = assert_raises(TypeError) { (.."c").max(2) }
      assert_equal "can't iterate from NilClass", e.message
    end
  end
end
