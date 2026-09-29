# rbs_inline: enabled
# args: --seed 1

require "strscan"
require "shellwords"
require "minitest/autorun"

# A tokenizer for a tiny arithmetic language, then an evaluator over its tokens.
class Lexer
  #: (String) -> void
  def initialize(src)
    @ss = StringScanner.new(src)
  end

  #: () -> Array[[Symbol, String]]
  def tokens
    out = [] #: Array[[Symbol, String]]
    until @ss.eos?
      if @ss.skip(/\s+/)
        next
      elsif (t = @ss.scan(/\d+(\.\d+)?/))
        out << [:num, t]
      elsif (t = @ss.scan(/[a-z_]\w*/))
        out << [:ident, t]
      elsif (t = @ss.scan(/\*\*|[-+*\/()=]/))
        out << [:op, t]
      else
        raise ArgumentError, "unexpected #{@ss.peek(1).inspect} at #{@ss.pos}"
      end
    end
    out
  end
end

class LexerTest < Minitest::Test
  def test_tokens
    assert_equal [[:ident, "x"], [:op, "="], [:num, "3"], [:op, "+"], [:num, "4.5"], [:op, "*"],
                  [:op, "("], [:ident, "y"], [:op, "**"], [:num, "2"], [:op, ")"]],
                 Lexer.new("x = 3 + 4.5 * (y ** 2)").tokens
  end

  def test_unexpected_character
    e = assert_raises(ArgumentError) { Lexer.new("1 + $").tokens }
    assert_equal "unexpected \"$\" at 4", e.message
  end
end

class StringScannerTest < Minitest::Test
  # One scanner walked step by step: each call moves (or doesn't move) the position.
  def test_scanning_walk
    ss = StringScanner.new("key: value; other: thing")
    assert_equal "key: value", ss.scan(/(\w+): (\w+)/)
    assert_equal ["key", "value", "key: value"], [ss[1], ss[2], ss[0]]
    assert_equal "key: value", ss.matched
    assert_equal true, ss.matched?
    assert_equal 10, ss.matched_size
    assert_equal [10, 10], [ss.pos, ss.charpos]
    assert_equal "; other: thing", ss.rest
    assert_equal 14, ss.rest_size
    assert_equal "", ss.pre_match
    assert_equal "; other: thing", ss.post_match
    assert_equal ["key", "value"], ss.captures

    # A failed scan clears the match but keeps the position.
    assert_nil ss.scan(/nope/)
    assert_equal false, ss.matched?
    assert_nil ss[0]

    # check looks ahead without advancing; scan_until advances past the match.
    assert_equal ";", ss.check(/;/)
    assert_equal 10, ss.pos
    assert_equal "; other", ss.scan_until(/other/)
    assert_equal "key: value; ", ss.pre_match
    assert_equal 17, ss.pos

    # skip_until/exist?/match? return lengths.
    assert_equal 5, ss.skip_until(/i/)
    assert_equal 2, ss.exist?(/g/)
    assert_equal 1, ss.match?(/n/)
    assert_equal 22, ss.pos

    assert_equal "n", ss.getch
    assert_equal "g", ss.get_byte
    assert_equal "", ss.peek(10)
    assert_equal true, ss.eos?
    ss.unscan
    assert_equal 23, ss.pos
  end

  def test_position_terminate_and_reset
    ss = StringScanner.new("key: value; other: thing")
    ss.pos = 5
    assert_equal "value; other: thing", ss.rest
    assert_equal false, ss.bol?
    assert_equal "value;", ss.check_until(/;/)
    ss.terminate
    assert_equal true, ss.eos?
    assert_equal "", ss.rest
    assert_nil ss.scan(/x/)
    ss.reset
    assert_equal 0, ss.pos
    assert_equal "key: value; other: thing", ss.string
  end

  # pos counts bytes, charpos counts characters.
  def test_multibyte_positions_and_inspect
    ss = StringScanner.new("")
    ss.string = "héllo wörld"
    assert_equal "hé", ss.scan(/h./)
    assert_equal 3, ss.pos
    assert_equal 2, ss.charpos
    assert_equal "llo wö", ss.scan_until(/ö/)
    assert_equal 8, ss.charpos
    assert_equal '#<StringScanner 10/13 "...o w\xC3\xB6" @ "rld">', ss.inspect
    ss.reset
    assert_equal '#<StringScanner 0/13 @ "h\xC3\xA9ll...">', ss.inspect
    ss.terminate
    assert_equal "#<StringScanner fin>", ss.inspect
  end

  # ^ anchors at the start of a line, not of the string.
  def test_beginning_of_line
    assert_equal true, StringScanner.new("a\nb").tap { |s| s.scan(/a\n/) }.beginning_of_line?
    assert_nil StringScanner.new("abc").scan(/^b/)
    assert_equal "b", StringScanner.new("abc").tap { |s| s.getch }.scan(/^b/)
  end
end

class ShellwordsTest < Minitest::Test
  def test_split_handles_quotes_and_escapes
    assert_equal ["cp", "my file.txt", "other dir/", "plain word", "xy zw"],
                 Shellwords.split(%q{cp "my file.txt" 'other dir/' plain\ word x"y z"w})
    assert_equal ["a", "b", "c"], "a  b\tc\n".shellsplit
    assert_equal ["esc \" $ \\ \\a"], Shellwords.shellwords(%q{"esc \" \$ \\ \a"})
  end

  def test_escape_and_join
    assert_equal "it\\'s\\ a\\ \\$test", Shellwords.escape("it's a $test")
    assert_equal "''", "".shellescape
    assert_equal "multi'\n'line", "multi\nline".shellescape
    assert_equal "h\\éllo", "héllo".shellescape
    assert_equal "ls -la a\\ b", ["ls", "-la", "a b"].shelljoin
    assert_equal 'x y\;z', Shellwords.join(["x", "y;z"])
  end

  def test_unmatched_quotes_raise
    e = assert_raises(ArgumentError) { Shellwords.split(%q{echo 'oops}) }
    assert_equal "Unmatched quote at 5: ...'", e.message
    e = assert_raises(ArgumentError) { Shellwords.split(%q{a "b}) }
    assert_equal "Unmatched quote at 2: ...\"", e.message
  end
end
