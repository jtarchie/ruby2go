# rbs_inline: enabled

require "minitest/autorun"

module RegexpTests
  # /x is stripped before the groups are counted and translated, wherever a pattern is built (decision 24).
  class RegexpExtendedTest < Minitest::Test
    def test_scan_groups_ignore_x_comments
      assert_equal [["a"], ["a"]], "a b a".scan(/(a) # (note)/x)
      assert_equal [["a", "b"]], "ab".scan(/(a) # (note)
                                           (b)/x)
      assert_equal [], "a b a".scan(/(a) b # (note)/x)
      assert_equal ["a", "a"], "a b a".scan(/a # (note)/x) # no groups: strings, not rows
    end

    def test_rooted_regexp_new_lookbehind
      assert ::Regexp.new("(?<=a)b").match?("ab")
      refute ::Regexp.new("(?<=a)b").match?("cb")
    end

    def test_interpolated_x_scan
      note = "(a) # (note)"
      assert_equal ["a"], "a a".match(/#{note}/x).captures
      assert_equal "a", "a".match(/#{note}/x)[1]
    end

    def test_regexp_new_extended_option
      re = Regexp.new("(a) # (note)\n b", Regexp::EXTENDED)
      assert_equal 2, re.options
      assert_equal "(a) # (note)\n b", re.source
      assert_equal ["a"], "ab".match(re).captures
      assert_nil "a b a".match(re)
      assert_equal "a-", "ab".sub(re, "\\1-")
      assert Regexp.new("a b", "x").match?("ab")
      assert_equal "/a b/mx", Regexp.new("a b", "mx").inspect
      assert_equal "/a b/ix", Regexp.new("a b", Regexp::EXTENDED | Regexp::IGNORECASE).inspect
      assert Regexp.new("a b", Regexp::EXTENDED | Regexp::IGNORECASE).match?("AB")
    end

    def test_timeout_error_class
      assert_equal RegexpError, Regexp::TimeoutError.superclass
    end

    # regexp2 patterns (lookaround) take Ruby's empty-match rule too
    def test_alt_find_all
      assert_equal "a-c", "abc".gsub(/(?<=a)b*/, "-")
      assert_equal "é |a\n|b é\n", "é a\nb é\n".gsub(/\b(?=\w)/, "|")
      # Onigmo's ^ does not match after a final newline (regexp2's would)
      assert_equal "> a\n", "a\n".gsub(/^(?=)/, "> ")
      assert_equal [""], "a\n".scan(/^(?!x)/)
      assert_equal "> a\n> b\n", "a\nb\n".gsub(/^(?=.)/, "> ")
      assert_equal false, "a\n".match?(/\n^(?!x)/)
      assert_equal true, "a\n^".match?(/a\n\^(?!x)/)
      assert_equal true, "ab".match?(/[^c]^?(?!x)b/)
    end
  end
end
