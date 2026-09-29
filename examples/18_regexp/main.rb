# rbs_inline: enabled
# args: --seed 1
# Regexp literals (Ruby syntax on Go's RE2), =~, !~, match, MatchData.

require "minitest/autorun"

#: (String) -> String
def format_of(path)
  case path
  when /\.html$/ then "html"
  when /\.json$/ then "json"
  else "other"
  end
end

class RegexpTest < Minitest::Test
  PATH = "/posts/42.json"

  # `=~` returns the match offset or nil; `!~` is its negation.
  #: () -> void
  def test_match_operators
    assert_equal 7, PATH =~ /\d+/
    assert_nil PATH =~ /zzz/
    assert_equal true, PATH !~ /\.html$/
    assert_equal false, PATH !~ /json/
  end

  # MatchData indexes groups; an unmatched optional group is nil.
  #: () -> void
  def test_match_data
    m = PATH.match(%r{^/(\w+)/(\d+)(\.\w+)?$})
    got = [m[0], m[1], m[2], m[3], m[4]] if m
    assert_equal ["/posts/42.json", "posts", "42", ".json", nil], got
    assert_nil PATH.match(/xml/)
  end

  #: () -> void
  def test_match_predicate_and_flags
    assert_equal true, PATH.match?(/json/)
    assert_equal false, "abc".match?(/^b/)
    assert_equal true, "Hello".match?(/hello/i)
    # `^` and `$` match at line boundaries, as in Ruby.
    assert_equal true, "line one\nline two".match?(/^line two$/)
  end

  #: () -> void
  def test_interpolated_regexp
    id = "42"
    assert_equal true, PATH.match?(%r{/#{id}(\.\w+)?/?$})
    assert_equal false, "/posts/43".match?(%r{/#{id}$})
  end

  # Offsets count characters, not bytes.
  #: () -> void
  def test_multibyte_offset
    assert_equal 6, "héllo wörld" =~ /w/
  end

  #: () -> void
  def test_case_when_regexp
    assert_equal "json", format_of(PATH)
    assert_equal "html", format_of("/a.html")
    assert_equal "other", format_of("/a")
  end
end
