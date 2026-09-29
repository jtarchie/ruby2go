# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

#: (Hash[String, String], String) -> String
def greeting(h, k)
  h[k]&.upcase || "DEFAULT"
end

class NilTest < Minitest::Test
  H = { "a" => "hi" }

  # `&.` skips the call on nil; `||` supplies the fallback.
  #: () -> void
  def test_safe_navigation_and_or
    assert_equal "HI", greeting(H, "a")
    assert_equal "DEFAULT", greeting(H, "b")
  end

  # `if name` narrows String? to String.
  #: () -> void
  def test_narrowing
    name = H["a"]
    size = name.size if name
    assert_equal 2, size
  end

  #: () -> void
  def test_inspect_on_optional
    assert_equal "\"hi\"", H["a"].inspect
    assert_equal "nil", H["zz"].inspect
  end
end
