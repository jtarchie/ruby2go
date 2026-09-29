# rbs_inline: enabled
# args: --seed 1
# Everyday Ruby semantics that untyped-looking code relies on: `rescue`
# modifiers, `return` inside `ensure`, NoMethodError on nil, `&&`/`||`
# returning values (and short-circuiting), locals first set in a branch,
# narrowing attribute reads, subclass-only methods, truthiness of untyped.

require "minitest/autorun"

#: () -> String
def swallow
  raise ArgumentError, "lost"
ensure
  return "ensure wins"
end

#: (bool) -> String
def maybe(flag)
  begin
    raise ArgumentError, "kept" if flag
    "body"
  ensure
    return "early" unless flag
  end
end

class Shape
  attr_reader :label #: String?

  #: (String?) -> void
  def initialize(label)
    @label = label
  end

  #: () -> String
  def describe
    return "unlabeled" unless label
    "shape #{label.upcase} (#{area})"
  end
end

class Square < Shape
  #: () -> Integer
  def area = 4
end

class RubySemanticsTest < Minitest::Test
  #: () -> void
  def test_rescue_modifier
    values = [1, 2]
    missing_value = values.fetch(5) rescue -1
    present_value = values.fetch(1) rescue -1
    assert_equal [-1, 2], [missing_value, present_value]
  end

  # `return` in `ensure` discards the exception; without it, it propagates.
  #: () -> void
  def test_return_in_ensure
    assert_equal "ensure wins", swallow
    assert_equal "early", maybe(false)
    e = assert_raises(ArgumentError) { maybe(true) }
    assert_equal "kept", e.message
  end

  #: () -> void
  def test_no_method_error_on_nil
    match = "abc".match(/z/)
    e = assert_raises(NoMethodError) { match[0] }
    assert_equal "undefined method '[]' for nil", e.message
  end

  # `&&` and `||` return an operand, and skip the right side when they can.
  #: () -> void
  def test_and_or_return_values
    log = []
    none = nil #: Integer?
    some = 5 #: Integer?
    assert_nil(none && none + 1)
    assert_equal 6, (some && some + 1)
    skipped = "a" == "b" && log.push("never").size > 0
    assert_equal false, skipped
    assert_equal 0, log.size
    missing = nil #: String?
    assert_equal 42, (missing || 42)
    assert_equal "yes", (true && "yes")
    assert_equal "fallback", (false || "fallback")
  end

  #: () -> void
  def test_local_first_set_in_branch
    values = [1, 2]
    if values.size > 1
      amount = "many"
    else
      amount = "few"
    end
    assert_equal "many", amount
  end

  # `area` exists only on Square; calling it on a bare Shape is a NameError.
  #: () -> void
  def test_subclass_only_method
    assert_equal "shape SQ (4)", Square.new("sq").describe
    assert_equal "unlabeled", Shape.new(nil).describe
    e = assert_raises(NameError) { Shape.new("bare").describe }
    assert_equal "NameError: undefined local variable or method 'area' for an instance of Shape", "#{e.class}: #{e.message}"
  end

  #: () -> void
  def test_untyped_truthiness
    flag = nil #: untyped
    assert_equal "no", (flag ? "yes" : "no")
    mixed = [1, nil, "x", false]
    assert_equal [1, "x"], mixed.select { |v| v }
  end
end
