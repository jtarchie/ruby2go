# rbs_inline: enabled

require "minitest/autorun"

# Union types (decision 150): `A | B` values, calls on them as per-member
# type switches, narrowing by is_a?/case/nil checks, and the untyped boundary.

#: (Integer | String) -> String
def union_show(x) = "#{x}!"

#: (Integer | String) -> Integer
def union_width(x) = x.size

#: (bool) -> (Integer | String)
def union_pick(b) = b ? 7 : "seven"

#: (Integer | Float) -> (Integer | Float)
def union_bump(n) = n + 1

#: (untyped) -> untyped
def union_untyped(v) = v

#: (bool) -> (String | Regexp)
def union_sep(b) = b ? "," : /;\s*/

#: (Integer) -> String
def union_int_only(n) = "int #{n}"

#: (bool) -> (Integer | Float)
def union_num(b) = b ? 2 : 0.5

# no annotations: these join to unions (decision 150), where they were errors
def union_ident(x) = x
def union_half(x) = x / 2
# @rbs b: bool
def union_pick_inferred(b) = b ? 1 : "x"

module UnionTests
  class Dog
    #: () -> String
    def speak = "woof"
  end

  class Robot
    #: () -> String
    def speak = "beep"
  end

  class Box
    attr_reader :v #: Integer | String

    #: (Integer | String) -> void
    def initialize(v)
      @v = v
    end

    #: () -> String
    def label
      case @v
      when Integer then "int #{@v + 1}"
      when String then "str #{@v.upcase}"
      end
    end
  end

  class Taker
    #: (Integer | String) -> String
    def take(x) = x.inspect
  end

  class UnionTest < Minitest::Test
    def test_calls_switch_on_member
      assert_equal "1!", union_show(1)
      assert_equal "a!", union_show("a")
      assert_equal 8, union_width(255)
      assert_equal 5, union_width("hello")
    end

    def test_union_result
      a = union_pick(true)
      b = union_pick(false)
      assert_equal 7, a
      assert_equal "seven", b
      assert_equal "7", a.to_s
      assert_equal "\"seven\"", b.inspect
      assert(a == 7)
      refute(b == 7)
    end

    #: (Integer | String | nil) -> String
    def nil_member(x) = x.inspect

    def test_nil_member
      assert_equal "nil", nil_member(nil)
      assert_equal "3", nil_member(3)
      assert_equal "\"s\"", nil_member("s")
    end

    #: (Integer | String | nil) -> String
    def truthy(x)
      if x
        x.to_s * 2
      else
        "none"
      end
    end

    def test_truthiness_drops_nil
      assert_equal "33", truthy(3)
      assert_equal "ss", truthy("s")
      assert_equal "none", truthy(nil)
    end

    #: (Integer | String) -> Integer
    def twice(x)
      if x.is_a?(Integer)
        x * 2
      else
        x.size * 2
      end
    end

    #: (Integer | String | Symbol) -> String
    def guard(x)
      return "int" if x.is_a?(Integer)
      return "str #{x.length}" unless x.is_a?(Symbol)

      "sym #{x.length}"
    end

    #: (Integer | String | Symbol) -> String
    def negated(x)
      if !x.is_a?(Integer)
        x.to_s
      else
        (x + 1).to_s
      end
    end

    #: (Integer | String | Symbol) -> Integer
    def either(x)
      if x.is_a?(String) || x.is_a?(Symbol)
        x.size
      else
        x
      end
    end

    #: (Integer | String | Symbol) -> String
    def unless_else(x)
      unless x.is_a?(Integer)
        x.to_s
      else
        (x * 3).to_s
      end
    end

    def test_narrowing
      assert_equal 8, twice(4)
      assert_equal 6, twice("abc")
      assert_equal "int", guard(1)
      assert_equal "str 3", guard("abc")
      assert_equal "sym 4", guard(:size)
      assert_equal "s", negated("s")
      assert_equal "2", negated(1)
      assert_equal 3, either("abc")
      assert_equal 2, either(:ab)
      assert_equal 9, either(9)
      assert_equal "z", unless_else(:z)
      assert_equal "6", unless_else(2)
    end

    #: (Integer | Float | String) -> String
    def classify(x)
      case x
      when Integer, Float then "number #{x * 2}"
      else "text #{x.upcase}"
      end
    end

    #: (Integer | Float | String | nil) -> String
    def classify_nil(x)
      case x
      when nil then "nothing"
      when String then x.reverse
      when Numeric then (x + 1).to_s
      end
    end

    def test_case_when
      assert_equal "number 4", classify(2)
      assert_equal "number 5.0", classify(2.5)
      assert_equal "text AB", classify("ab")
      assert_equal "nothing", classify_nil(nil)
      assert_equal "ba", classify_nil("ab")
      assert_equal "2", classify_nil(1)
      assert_equal "2.5", classify_nil(1.5)
    end

    def test_numeric_union
      assert_equal 2, union_bump(1)
      assert_in_delta 2.5, union_bump(1.5)
      assert_equal "Float", union_bump(0.5).class.name
    end

    def test_struct_members
      pets = [Dog.new, Robot.new] #: Array[Dog | Robot]
      assert_equal %w[woof beep], pets.map { |p| p.speak }
    end

    def test_containers
      items = [] #: Array[Integer | String]
      items << 1
      items << "two"
      items.push(3)
      assert_equal [1, "two", 3], items
      assert_equal %w[1 two 3], items.map { |i| i.to_s }
      assert_includes items, "two"
      h = {} #: Hash[Symbol, Integer | String]
      h[:a] = 1
      h[:b] = "b"
      assert_equal({ a: 1, b: "b" }, h)
      v = h.fetch(:a)
      assert_equal 2, twice(v) / 1
    end

    def test_ivar_and_reader
      assert_equal "int 42", Box.new(41).label
      assert_equal "str HI", Box.new("hi").label
      assert_equal "hi", Box.new("hi").v
    end

    #: (Array[Integer] | String) -> Integer
    def total(xs)
      return xs.size if xs.is_a?(String)

      sum = 0
      xs.each { |x| sum += x }
      sum
    end

    #: (Array[Integer] | Range[Integer]) -> Integer
    def iterate(xs)
      sum = 0
      xs.each { |x| sum += x }
      sum
    end

    def test_iterators
      assert_equal 6, total([1, 2, 3])
      assert_equal 5, total("hello")
      assert_equal 6, iterate([1, 2, 3])
      assert_equal 10, iterate(1..4)
    end

    def test_untyped_boundary
      assert_equal 14, twice(union_untyped(7))
      assert_equal 4, twice(union_untyped("ab"))
      raised = false
      begin
        twice(union_untyped(1.5)) # rb2go: TypeError at the boundary (decision 20); MRI: NoMethodError inside
      rescue TypeError, NoMethodError
        raised = true
      end
      assert raised
    end

    def test_dynamic_call
      t = Taker.new #: untyped
      assert_equal "5", t.take(5)
      assert_equal "\"x\"", t.take("x")
    end

    #: (bool) -> ([Integer, String] | Integer)
    def pair_or_int(b) = b ? [1, "a"] : 2

    #: (bool) -> (Hash[Symbol, Integer] | String)
    def hash_or_str(b) = b ? { a: 1 } : "none"

    def test_literal_members
      v = pair_or_int(true)
      assert_equal [1, "a"], v
      assert_equal 2, pair_or_int(false)
      assert_equal "[1, \"a\"]", v.inspect
      assert v.is_a?(Array)
      both = [v, 3] #: Array[untyped]
      assert_equal [[1, "a"], 3], both
      assert_equal({ a: 1 }, hash_or_str(true))
      assert_equal "none", hash_or_str(false)
    end

    def test_joins_without_annotations
      x = 3
      a = case x
          when 1 then "one"
          else 0
          end
      assert_equal 0, a
      b = x > 1 ? 1 : "s"
      assert_equal 1, b
      c = x > 5 ? 1 : "s"
      assert_equal "s", c.to_s
      v = 1
      v = "s" if x > 1
      assert_equal "s", v
      w = if x.odd?
            "odd"
          else
            2
          end
      assert_equal "odd", w
      assert_equal 1, union_pick_inferred(true)
      assert_equal "x", union_pick_inferred(false)
      assert_equal 1, union_ident(1)
      assert_equal "a", union_ident("a")
      assert_equal 2, union_half(4)
      assert_in_delta 1.25, union_half(2.5)
      id = ->(z) { z }
      assert_equal 1, id.call(1)
      assert_equal "s", id.call("s")
      total = 0
      total += 1.5
      total += 2
      assert_in_delta 3.5, total
    end

    #: (Integer) -> (Integer | String | nil)
    def maybe(i) = i > 1 ? i : (i.zero? ? nil : "s")

    def test_or_and_on_unions
      y = maybe(0)
      y ||= "default"
      assert_equal "default", y
      z = maybe(5)
      z ||= "d"
      assert_equal 5, z
      assert_in_delta 1.5, maybe(0) || 1.5
      assert_equal :yes, maybe(1) && :yes
      assert_nil maybe(0) && :yes
      m = maybe(7)
      assert_equal "7", m && m.to_s
    end

    def test_union_arguments
      assert_equal ["a", "b;c"], "a,b;c".split(union_sep(true))
      assert_equal ["a,b", "c"], "a,b; c".split(union_sep(false))
      assert_equal "a!b", "a,b".sub(union_sep(true), "!")
      assert_equal 20, 10 * union_num(true)
      assert_in_delta 5.0, 10 * union_num(false)
      assert_in_delta 5.0, 10.0 / union_num(true)
      order = [] #: Array[String]
      pick = ->(s) { order << s; s }
      assert_equal "xy", pick.call("x") + pick.call("y")
      assert_equal %w[x y], order
    end

    #: (Integer | String) -> String
    def routed(x) = x.is_a?(Integer) ? union_int_only(x) : x

    def test_union_argument_needs_narrowing
      assert_equal "int 1", routed(1)
      assert_equal "s", routed("s")
    end

    #: (Integer) -> (Integer | String | Array[Integer] | nil)
    def varied(i)
      case i
      when 0 then nil
      when 1 then 5
      when 2 then "five"
      else [1, 2]
      end
    end

    #: (Integer | String | Array[Integer] | nil) -> String
    def matched(v)
      case v
      in Integer => n then "int #{n + 1}"
      in String => s then "str #{s.upcase}"
      in [a, b] then "pair #{a} #{b}"
      in nil then "nil"
      end
    end

    def test_patterns
      assert_equal ["nil", "int 6", "str FIVE", "pair 1 2"], (0..3).map { |i| matched(varied(i)) }
      assert varied(0).is_a?(NilClass)
      refute varied(1).is_a?(NilClass)
      assert varied(0).nil?
    end

    #: (Integer | bool) -> String
    def flag(x)
      x ? "yes #{x}" : "no"
    end

    def test_boolean_member
      assert_equal "yes 1", flag(1)
      assert_equal "yes true", flag(true)
      assert_equal "no", flag(false)
      b = true #: Integer | bool
      assert b.is_a?(TrueClass)
      refute b.is_a?(FalseClass)
    end
  end
end
