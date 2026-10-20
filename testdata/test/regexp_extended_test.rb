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

  # #102: grep/grep_v with a block set $~ for each test, as MRI's
  class RegexpGrepLastMatchTest < Minitest::Test
    def test_grep_block_last_match
      assert_equal ["b", "b"], ["aba", "aba"].grep(/a(b)a/) { $1 }
      "z" =~ /z/
      got = ["abc", "def"].grep(/b/) { |e| [e, $&] }
      assert_equal [[["abc", "b"]], nil], [got, $~]
      "z" =~ /z/
      assert_equal [["abc"], "z"], [["abc", "def"].grep(/b/), $&]
      inv = ["abc", "def"].grep_v(/e/) { |e| [e, $~.nil?] }
      assert_equal [[["abc", true]], "e"], [inv, $&]
    end

    # `::Regexp.last_match(n)` reads the frame's $~ as `Regexp.last_match(n)` does (Homebrew's expand_elf_dst).
    def test_rooted_last_match
      out = "$ORIGIN/lib:${LIB}".gsub(/\$({?)([A-Z]+)(}?)/) { "<#{::Regexp.last_match(2)}#{::Regexp.last_match(1).empty?}>" }
      assert_equal "<ORIGINtrue>/lib:<LIBfalse>", out
    end

    def test_escape_symbol
      assert_equal ["a\\.b", "x\\+"], [Regexp.escape(:"a.b"), Regexp.quote("x+")]
    end

    # A binary String (not valid UTF-8) matches byte by byte (decision 324): Homebrew's magic-number checks.
    def test_binary_subject
      data = "\xFD7zXZ\x00\x01\x02".b
      assert_equal [true, 2, "z", true, false], [data.match?(/\A\xFD7zXZ\x00/n), data =~ /zX/n, data.match(/7(z)X/n)&.[](1),
                                              /\A\xFD/n === data, "\x1f\x8b\xff".b.match?(/\A\xFD/n)]
    end
  end
end
