# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class StringsTest < Minitest::Test
  S = "Hello, World"

  #: () -> void
  def test_case_and_size
    assert_equal 12, S.size
    assert_equal "HELLO, WORLD", S.upcase
    assert_equal "hello, world", S.downcase
    assert_equal "dlroW ,olleH", S.reverse
  end

  #: () -> void
  def test_predicates
    assert_equal true, S.include?("World")
    assert_equal true, S.start_with?("Hell")
    assert_equal false, S.end_with?("x")
  end

  # `index` and `[]` return nil when nothing is there.
  #: () -> void
  def test_index_and_slicing
    assert_equal 4, S.index("o")
    assert_nil S.index("zz")
    assert_equal "H", S[0]
    assert_equal "d", S[-1]
    assert_nil S[99]
    assert_equal ["H", "e", "l"], S.chars.first(3)
  end

  #: () -> void
  def test_substitution
    assert_equal "HeLlo, World", S.sub("l", "L")
    assert_equal "HeLLo, WorLd", S.gsub("l", "L")
    assert_equal "He001, W1r0d", S.tr("lo", "01")
  end

  #: () -> void
  def test_strip_repeat_split
    assert_equal "padded|", "  padded  ".strip + "|"
    assert_equal "abcabcabc", "abc" * 3
    assert_equal ["a", "b", "c"], "a-b-c".split("-")
    assert_equal ["one", "two", "three"], "one two  three".split
  end

  #: () -> void
  def test_padding
    assert_equal "  x  |", "x".center(5) + "|"
    assert_equal "x  |", "x".ljust(3) + "|"
    assert_equal "  x|", "x".rjust(3) + "|"
  end

  # `to_i`/`to_f` parse a numeric prefix.
  #: () -> void
  def test_conversions
    assert_equal 43, "42abc".to_i + 1
    assert_equal 7.0, "3.5".to_f * 2
    assert_equal 97, "ab".ord
    assert_equal "a", 97.chr
  end

  #: () -> void
  def test_inspect_escapes
    assert_equal '"tab\there"', "tab\there".inspect
    assert_equal '"quote\"d"', "quote\"d".inspect
    assert_equal '"new\nline"', "new\nline".inspect
  end

  # Interpolation calls `to_s`; nil becomes "".
  #: () -> void
  def test_interpolation
    assert_equal "%s2true2.5", "%s" + "#{1 + 1}" + "#{true}" + "#{nil}" + "#{2.5}"
  end

  #: () -> void
  def test_each_char
    count = 0
    "mississippi".each_char { |c| count += 1 if c == "s" }
    assert_equal 4, count
  end

  #: () -> void
  def test_capitalize
    assert_equal "Ruby", "Ruby".capitalize
    assert_equal "Ruby", "rUBY".capitalize
  end

  #: () -> void
  def test_join_and_array_inspect
    assert_equal "x, y", ["x", "y"].join(", ")
    assert_equal "[1, [2, 3]]", [1, [2, 3]].inspect
    assert_equal "[]", [].inspect
  end

  #: () -> void
  def test_comparison
    assert_equal(-1, "abc" <=> "abd")
    assert_equal true, "b" > "a"
    assert_equal "b", "a".clamp("b", "c")
  end
end
