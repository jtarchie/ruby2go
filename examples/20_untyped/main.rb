# rbs_inline: enabled
# args: --seed 1
# Working with `untyped`: is_a? checks narrow a local, `||` and truthiness.

require "minitest/autorun"

#: (untyped) -> String
def render(resource)
  return "none" unless resource
  if resource.is_a?(Array)
    "[" + resource.map { |r| render(r) }.join(",") + "]"
  elsif resource.is_a?(Integer)
    "int:#{resource + 1}"
  elsif resource.is_a?(String)
    "str:#{resource.upcase}"
  else
    "<#{resource}>"
  end
end

class UntypedTest < Minitest::Test
  # Each `is_a?` branch narrows `resource` to that class.
  #: () -> void
  def test_is_a_narrowing
    assert_equal ["none", "none", "int:6", "str:S", "[int:2,str:X,[int:3]]", "<2.5>"],
                 [render(nil), render(false), render(5), render("s"), render([1, "x", [2]]), render(2.5)]
  end

  #: () -> void
  def test_or_on_untyped
    value = nil #: untyped
    assert_equal "fallback", value || "fallback"
    assert_equal 3, value || 3
  end

  #: () -> void
  def test_ancestry_checks
    assert_equal true, 5.is_a?(Integer)
    assert_equal true, 5.is_a?(Comparable)
    assert_equal false, "s".is_a?(Integer)
    assert_equal true, 5.kind_of?(Object)
  end
end
