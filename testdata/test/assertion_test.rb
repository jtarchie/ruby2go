# rbs_inline: enabled

require "minitest/autorun"
require "set"

# Typed minitest assertions (issue #29, decision 93): each assertion with
# typed, mixed T/T? and untyped arguments, its failure message, and the
# assertion count it adds, all as MRI's minitest gives them.
module AssertionTests
  class Box
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
    end

    #: (untyped) -> bool
    def ==(other) = other.is_a?(Box) && other.v == v

    #: () -> String
    def inspect = "Box(#{v})"
  end

  class TypedAssertionTest < Minitest::Test
    #: (^() -> void) -> String
    def failure(check)
      assert_raises(Minitest::Assertion) { check.call }.message
    end

    #: (^() -> void) -> Integer
    def counted(check)
      before = assertions
      check.call
      assertions - before
    end

    def test_equal_mixed_optional
      h = { a: 3 } #: Hash[Symbol, Integer]
      assert_equal 3, h[:a]
      assert_equal h[:a], 3
      assert_equal "Expected: 3\n  Actual: nil", failure(-> { assert_equal 3, h[:b] })
      assert_equal "Expected: 4\n  Actual: 3", failure(-> { assert_equal 4, h[:a] })
      assert_equal "Expected 3 to not be equal to 3.", failure(-> { refute_equal h[:a], 3 })
      refute_equal 4, h[:a]
      assert_equal Box.new(1), Box.new(1)
      assert_equal "Expected: Box(1)\n  Actual: Box(2)", failure(-> { assert_equal Box.new(1), Box.new(2) })
      assert_equal "Expected Box(1) to not be equal to Box(1).", failure(-> { refute_equal Box.new(1), Box.new(1) })
      assert_equal 1, 1.0
      assert_equal [1, 2], [1, 2]
    end

    def test_same
      s = :sym
      assert_same :sym, s
      refute_same "a", "a".dup
      msg = failure(-> { assert_same "a", "a".dup })
      assert_match(/\AExpected "a" \(oid=\d+\) to be the same as "a" \(oid=\d+\)\.\z/, msg)
    end

    def test_in_delta
      assert_in_delta 1.0, 1.0001, 0.01
      assert_equal "Expected |1.0 - 1.5| (0.5) to be <= 0.1.", failure(-> { assert_in_delta 1.0, 1.5, 0.1 })
      refute_in_delta 1.0, 1.5, 0.1
      assert_equal "Expected |1.0 - 1.05| (0.050000000000000044) to not be <= 0.1.", failure(-> { refute_in_delta 1.0, 1.05, 0.1 })
    end

    def test_includes
      assert_includes [1, 2], 2
      assert_includes({ a: 1 }, :a)
      assert_includes "hello", "ell"
      assert_includes Set[1, 2], 1
      refute_includes [1, 2], 3
      refute_includes "hello", "z"
      assert_equal "Expected [1, 2] to include 3.", failure(-> { assert_includes [1, 2], 3 })
      assert_equal "Expected {a: 1} to not include :a.", failure(-> { refute_includes({ a: 1 }, :a) })
      assert_equal 2, counted(-> { assert_includes [1], 1 })
      assert_equal 2, counted(-> { refute_includes "ab", "c" })
    end

    def test_empty
      assert_empty []
      assert_empty({})
      assert_empty ""
      refute_empty [1]
      refute_empty "x"
      assert_equal "Expected [1] to be empty.", failure(-> { assert_empty [1] })
      assert_equal "Expected \"\" to not be empty.", failure(-> { refute_empty "" })
      assert_equal 2, counted(-> { assert_empty Set.new([1]).select(&:zero?) })
    end

    def test_operator_and_predicate
      assert_operator 1, :<, 2
      assert_operator "b", :>, "a"
      refute_operator 3, :<, 2
      assert_operator 4, :even?
      assert_predicate 3, :odd?
      refute_predicate [1], :empty?
      assert_equal "Expected 3 to be < 2.", failure(-> { assert_operator 3, :<, 2 })
      assert_equal "Expected 1 to not be < 2.", failure(-> { refute_operator 1, :<, 2 })
      assert_equal "Expected 3 to be even?.", failure(-> { assert_predicate 3, :even? })
      assert_equal "Expected [] to not be empty?.", failure(-> { refute_predicate [], :empty? })
      assert_equal 2, counted(-> { assert_operator 1, :<=, 1 })
      assert_equal 2, counted(-> { assert_predicate 0, :zero? })
    end

    def test_respond_to
      assert_respond_to "x", :upcase
      refute_respond_to 1, :upcase
      assert_equal "Expected 1 (Integer) to respond to #upcase.", failure(-> { assert_respond_to 1, :upcase })
      assert_equal "Expected \"x\" to not respond to upcase.", failure(-> { refute_respond_to "x", :upcase })
      assert_equal 1, counted(-> { assert_respond_to [], :each })
    end

    # assert_raises(K) / must_raise(K) with one class literal is typed K (decision 116, #37).
    def test_raises_is_typed
      e = assert_raises(KeyError) { { a: 1 }.fetch(:b) }
      assert_equal :b, e.key
      assert_equal({ a: 1 }, e.receiver)
      t = assert_raises(UncaughtThrowError) { throw :nope }
      assert_equal :nope, t.tag
      f = assert_raises(FrozenError) { [1].freeze << 2 }
      assert_equal "can't modify frozen Array: [1]", f.message
      x = assert_raises(SystemExit) { exit 3 }
      assert_equal 3, x.status
      either = assert_raises(KeyError, IndexError) { [].fetch(5) }
      assert_equal "IndexError", either.class.name
    end

    # $stdout/$stderr assignment, capture_io, assert_output and assert_silent (decision 109).
    def test_output
      assert_output("hi\n") { puts "hi" }
      assert_output(nil, "e\n") { $stderr.puts "e" }
      assert_output(/h.+!/, "\n") { print "hello!"; warn "" }
      assert_output("\"a 1\"\n", /^w/) { p "a 1"; warn "warned" }
      assert_silent { nil }
      out, err = capture_io do
        puts "out"
        $stderr.print "err"
      end
      assert_equal ["out\n", "err"], [out, err]
      assert_equal "In stdout.\nExpected: \"x\"\n  Actual: \"y\"", failure(-> { assert_output("x") { print "y" } })
      assert_equal "In stderr.\nExpected /z/ to match \"\".", failure(-> { assert_output(nil, /z/) { nil } })
      assert_equal 2, counted(-> { assert_output("", "") { nil } })
      assert_equal 2, counted(-> { assert_silent { nil } })
      sio = StringIO.new
      $stdout = sio
      begin
        puts "redirected"
        printf("%d%%", 5)
        STDOUT.print "!"
      ensure
        $stdout = STDOUT
      end
      assert_equal "redirected\n5%", sio.string # STDOUT itself is not redirected
      e = assert_raises(TypeError) { $stdout = 1 }
      assert_equal "$stdout must have write method, Integer given", e.message
    end
  end
end
