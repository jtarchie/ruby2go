# rbs_inline: enabled

require "minitest/autorun"

# Pattern matching (issue #38, decision 143): every pattern form, deconstruct/deconstruct_keys and MRI's failure messages.
module PatternTests
  Point = Data.define(:x, :y) #: [Integer, Integer]
  Pair = Struct.new(:left, :right) #: [Integer, String]

  class Vec
    attr_reader :x #: Integer
    attr_reader :y #: Integer

    #: (Integer, Integer) -> void
    def initialize(x, y)
      @x = x
      @y = y
    end

    #: () -> Array[Integer]
    def deconstruct = [x, y]

    #: (Array[Symbol]?) -> Hash[Symbol, Integer]
    def deconstruct_keys(_keys) = { x: x, y: y }
  end

  class Shape; end

  class Circle < Shape
    attr_reader :r #: Integer

    #: (Integer) -> void
    def initialize(r)
      @r = r
    end

    #: (Array[Symbol]?) -> Hash[Symbol, Integer]
    def deconstruct_keys(_keys) = { r: r }
  end

  class Rect < Shape
    attr_reader :w #: Integer
    attr_reader :h #: Integer

    #: (Integer, Integer) -> void
    def initialize(w, h)
      @w = w
      @h = h
    end

    #: () -> Array[Integer]
    def deconstruct = [w, h]
  end

  class Odd
    #: () -> untyped
    def deconstruct = 5
  end

  class PatternTest < Minitest::Test
    #: (untyped) -> String
    def show(v)
      case v
      in Integer => n if n > 100 then "big #{n}"
      in [x, y] then "pair #{x},#{y}"
      in { name: String => name, age: } then "#{name} is #{age}"
      in Point(x:, y:) then "point #{x}/#{y}"
      in nil then "nothing"
      else "other"
      end
    end

    #: (Shape) -> Integer
    def area(s)
      case s
      in Circle(r:) then 3 * r * r
      in Rect[w, h] then w * h
      end
    end

    #: (Shape) -> Integer
    def size(s)
      case s
      in Circle then s.r
      in Rect then s.w + s.h
      end
    end

    #: (untyped) -> String
    def error_of(v)
      v => [Integer => a, *]
      a.to_s
    rescue NoMatchingPatternError => e
      e.message
    end

    def test_case_in
      assert_equal "big 500", show(500)
      assert_equal "other", show(7)
      assert_equal "pair 1,2", show([1, 2])
      assert_equal "Ann is 3", show({ name: "Ann", age: 3 })
      assert_equal "other", show({ name: 1, age: 3 })
      assert_equal "pair 1,2", show(Point.new(1, 2)) # Data#deconstruct comes first
      assert_equal "nothing", show(nil)
      assert_equal "other", show("x")
    end

    def test_value_patterns
      pin = 5
      got = [5, 6, "hello", 2.5, :sym, 42].map do |v|
        case v
        in ^pin then "pinned"
        in ^(pin + 1) then "pinned expr"
        in /ell/ then "regex"
        in 2.0..3.0 then "range"
        in :sym | :other then "symbol"
        else "other"
        end
      end
      assert_equal ["pinned", "pinned expr", "regex", "range", "symbol", "other"], got
      assert_equal true, (5 in 1..9)
      assert_equal false, (5 in ..3)
      assert_equal true, (5 in Integer | Float)
      assert_equal false, ("5" in Integer | Float)
    end

    def test_nil_and_optional
      x = nil #: Integer?
      got = case x
            in nil then "nil"
            in Integer => n then n.to_s
            end
      assert_equal "nil", got
      y = 4 #: Integer?
      inc = case y
            in nil then -1
            in Integer => n then n + 1
            end
      assert_equal 5, inc
    end

    def test_typed_bindings
      ints = [5, 6, 7, 8] #: Array[Integer]
      case ints
      in [Integer => a, Integer => b, *] if a < b
        assert_equal 30, a * b
      end
      case ints
      in [first, *mid, last]
        assert_equal 13, first + last
        assert_equal [6, 7], mid
      end
      case ints
      in [_, _, *] => all
        assert_equal 4, all.size
      end
      tup = [1, "a"] #: [Integer, String]
      tup => [i, s]
      assert_equal "A", s.upcase
      assert_equal 2, i + 1
      deep = [1, [2, [3, 4]]] #: untyped
      deep => [a1, [b1, [c1, d1]]]
      assert_equal 10, a1 + b1 + c1 + d1
    end

    def test_find_pattern
      ints = [5, 6, 7, 8] #: Array[Integer]
      case ints
      in [*pre, 7 => seven, *post]
        assert_equal [5, 6], pre
        assert_equal 7, seven
        assert_equal [8], post
      end
      mixed = [1, "a", :b, 2.0] #: untyped
      case mixed
      in [*, Symbol => sym, *rest]
        assert_equal :b, sym
        assert_equal [2.0], rest
      end
      assert_equal false, (ints in [*, 9, *])
      assert_equal true, (ints in [*, 6, 7, *])
    end

    def test_hash_patterns
      h = { a: 1, b: 2 } #: Hash[Symbol, Integer]
      case h
      in { a: Integer => a1, **rest }
        assert_equal 1, a1
        assert_equal({ b: 2 }, rest)
      end
      assert_equal false, (h in { a: 1, **nil })
      assert_equal true, (h in { a: 1, b: 2, **nil })
      assert_equal true, (h in { a: 1 })
      assert_equal false, (h in { c: 1 })
      assert_equal false, (h in {})
      e = {} #: Hash[Symbol, Integer]
      assert_equal true, (e in {})
      json = { status: "ok", items: [1, 2] } #: untyped
      case json
      in { status: "ok", items: [_, *] => items }
        assert_equal [1, 2], items
      end
      config = { port: 8080 } #: Hash[Symbol, Integer]
      config => { port: }
      assert_equal 8081, port + 1
    end

    def test_struct_and_data
      pair = Pair.new(1, "one")
      case pair
      in [l, r]
        assert_equal 2, l + 1
        assert_equal "ONE", r.upcase
      end
      case pair
      in { left: 1 | 2 => l2, right: }
        assert_equal [1, "one"], [l2, right]
      end
      pair => Pair[left, right2]
      assert_equal [1, "one"], [left, right2]
      pt = Point.new(x: 3, y: 4)
      got = case pt
            in { x: 0 } then 0
            in Point[a, b] then a * b
            end
      assert_equal 12, got
      assert_equal true, (pt in { x: 3, y: 4, **nil })
      assert_equal false, (pt in { x: 3, **nil })
      case pt
      in { x:, **rest }
        assert_equal 3, x
        assert_equal({ y: 4 }, rest)
      end
      assert_equal false, (pt in { z: 1 })
      assert_equal [3, 4], pt.deconstruct
      assert_equal({ y: 4, x: 3 }, pt.deconstruct_keys([:y, :x]))
      assert_equal({ x: 3 }, pt.deconstruct_keys([:x, :z]))
      assert_equal({}, pt.deconstruct_keys([:x, :y, :z]))
      assert_equal({ left: 1, right: "one" }, pair.deconstruct_keys(nil))
      assert_equal [1, "one"], pair.deconstruct
    end

    def test_user_deconstruct
      vec = Vec.new(3, 4)
      case vec
      in [x, y] then assert_equal 12, x * y
      end
      case vec
      in { x:, y: 4.. } then assert_equal 3, x
      end
      assert_equal true, (vec in Vec(x: 3))
      assert_equal false, (vec in Vec[_, 5])
      assert_equal 12, area(Rect.new(3, 4))
      assert_equal 27, area(Circle.new(3))
      assert_equal 3, size(Circle.new(3))
      assert_equal 7, size(Rect.new(3, 4))
    end

    def test_guards
      got = [1, 2, 3, 4].map do |v|
        case v
        in Integer => n if n.even? then "even"
        in Integer => n unless n > 2 then "small odd"
        else "other"
        end
      end
      assert_equal ["small odd", "even", "other", "even"], got
    end

    def test_in_and_rightward
      assert_equal true, (1 in Integer)
      assert_equal false, (1 in String)
      matched = ([1, 2] in [Integer => a, _])
      assert_equal true, matched
      assert_equal 1, a
      [1, 2] => [_, second]
      assert_equal 2, second
    end

    def test_single_pattern_messages
      assert_equal "1", error_of([1, 2])
      assert_equal "[]: [] length mismatch (given 0, expected 1+)", error_of([])
      assert_equal "5: 5 does not respond to #deconstruct", error_of(5)
      assert_equal "nil: nil does not respond to #deconstruct", error_of(nil)
      assert_equal "[\"a\"]: Integer === \"a\" does not return true", error_of(["a"])
      h = { a: 1, b: 2 } #: Hash[Symbol, Integer]
      begin
        h => { a: String }
      rescue NoMatchingPatternError => e
        assert_equal "{a: 1, b: 2}: String === 1 does not return true", e.message
      end
      begin
        h => { a: 1, **nil }
      rescue NoMatchingPatternError => e
        assert_equal "{a: 1, b: 2}: rest of {b: 2} is not empty", e.message
      end
      begin
        h => { a: 2, c: }
      rescue NoMatchingPatternKeyError => ke
        assert_equal "{a: 1, b: 2}: key not found: :c", ke.message
        assert_equal :c, ke.key
        assert_equal({ a: 1, b: 2 }, ke.matchee)
      end
      begin
        [5, 6] => [*, 9, *]
      rescue NoMatchingPatternError => e
        assert_equal "[5, 6]: [5, 6] does not match to find pattern", e.message
      end
      begin
        case 5
        in String | Float then 1
        end
      rescue NoMatchingPatternError => e
        assert_equal "5: Float === 5 does not return true", e.message
      end
      begin
        case 5
        in Integer if false then 1
        end
      rescue NoMatchingPatternError => e
        assert_equal "5: guard clause does not return true", e.message
      end
      begin
        1 => ^(2)
      rescue NoMatchingPatternError => e
        assert_equal "1: 2 === 1 does not return true", e.message
      end
      begin
        Point.new(1, 2) => { z: }
      rescue NoMatchingPatternKeyError => ke
        assert_equal "#<data PatternTests::Point x=1, y=2>: key not found: :z", ke.message
      end
    end

    def test_multi_arm_message
      err = nil #: NoMatchingPatternError?
      begin
        case 5
        in String then 1
        in Float then 2
        end
      rescue NoMatchingPatternError => e
        err = e
      end
      assert_equal "5", err&.message
      assert_equal NoMatchingPatternError, err&.class
    end

    def test_static_arms
      n = 5 #: Integer
      got = case n
            in String then "s"
            in Integer => i then "i#{i}"
            end
      assert_equal "i5", got
      h = { k: 1 } #: Hash[Symbol, Integer]
      if h in { k: Integer => kv }
        assert_equal 2, kv * 2
      end
      err = nil #: Exception?
      begin
        case Odd.new
        in [x] then x
        end
      rescue TypeError => e
        err = e
      end
      assert_equal TypeError, err&.class
    end

    def test_error_classes
      assert_equal true, NoMatchingPatternKeyError.new("x").is_a?(NoMatchingPatternError)
      assert_equal true, NoMatchingPatternError.new("x").is_a?(StandardError)
      begin
        NoMatchingPatternKeyError.new("x").key
      rescue ArgumentError => e
        assert_equal "no key is available", e.message
      end
    end
  end
end
