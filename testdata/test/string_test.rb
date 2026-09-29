# rbs_inline: enabled

require "minitest/autorun"

# Helpers for the checks that were testdata/run/string_literals.rb.
class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: () -> String
  def to_s = "Pt(#{x})"
end

# Helpers for the checks that were testdata/run/string_mid.rb.
# default inspect stays distinct from a user to_s
class MidPt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: () -> String
  def to_s = "MidPt(#{x})"
end

# a module's #{self} and bare to_s/inspect dispatch to the includer
module Tagged
  #: () -> String
  def tag = "[#{self}]"

  #: () -> String
  def loud = to_s.upcase

  #: () -> String
  def shown = "<#{inspect}>"
end

class Item
  include Tagged

  #: () -> String
  def to_s = "item"

  #: () -> String
  def inspect = "#<Item>"
end

# Helpers for the checks that were testdata/run/string_split.rb.
#: (String) -> Integer
def vowels(s)
  s.each_char { |c| return 1 if "aeiou".include?(c) }
  0
end

# Helpers for the checks that were testdata/run/string_symbol.rb.
#: (Hash[Symbol, String]) -> String
def greet(opts) = "hi #{opts[:name]}"

# Helpers for the checks that were testdata/run/string_dynamic.rb.
#: (untyped) -> untyped
def ident(v) = v

# Helpers for the checks that were testdata/run/string_bugs.rb.
# Kernel#to_s/inspect show a heap address
class Foo
end

# a literal passed to a generic param keeps String
class Fmt
  # @rbs [T] (T) -> String
  def show(x) = "<#{x}>|#{x.inspect}"
end

#: (Object) -> String
def kind(o) = o.class.name

#: (Object) -> String
def desc(o) = "#{o}|#{o.inspect}|#{o.is_a?(String)}|#{o.is_a?(Symbol)}"

#: (untyped) -> untyped
def ident_class(v) = v

#: (untyped) -> untyped
def ident_cmp(v) = v

# Helpers for the checks that were testdata/run/string_nil_receiver.rb.
#: (String?) -> String
def label(v)
  return "none" unless v

  v.capitalize
end

class StringConvertTest < Minitest::Test
  # to_i reads an optional sign and leading digits after whitespace; anything else is 0.
  def test_to_i_reads_an_optional
    assert_equal 42, "42abc".to_i
    assert_equal 42, " 42".to_i
    assert_equal -7, "-7".to_i
    assert_equal 5, "+5".to_i
    assert_equal 0, "abc".to_i
    assert_equal 0, "".to_i
    assert_equal 12, "12e3".to_i
    assert_equal 0, "0x1A".to_i
    assert_equal 9, "\n 9 \n".to_i
    assert_equal 0, "- 4".to_i
    assert_equal 0, "--4".to_i
    assert_equal 7, "007".to_i
    assert_equal 0, "-0".to_i
    assert_equal 3, "3.9".to_i
    assert_equal 9223372036854775807, "9223372036854775807".to_i
    assert_equal -9223372036854775808, "-9223372036854775808".to_i
    assert_equal 43, ("42".to_i + 1)
    assert_equal 12, "12 34".to_i
  end

  # to_f on well-formed input, including exponents and leading dots.
  def test_to_f_on_well_formed
    assert_equal 3.5, "3.5".to_f
    assert_equal 0.0, "abc".to_f
    assert_equal 1000.0, "1e3".to_f
    assert_equal 0.5, ".5".to_f
    assert_equal 2.25, "  2.25  ".to_f
    assert_equal -0.0, "-0.0".to_f
    assert_equal 0.0, "0x1A".to_f
    assert_equal 0.0, "".to_f
    assert_equal 1.0, "1.".to_f
    assert_equal -0.0015, "-1.5e-3".to_f
    assert_equal 10.0, "10".to_f
    assert_equal 1000.5, "1_000.5".to_f
    assert_equal 0.30000000000000004, ("0.1".to_f + "0.2".to_f)
  end

  # ord is the first character's codepoint.
  def test_ord_is_the_first_character
    assert_equal 97, "a".ord
    assert_equal 233, "é".ord
    assert_equal 128512, "😀".ord
    assert_equal 97, "abc".ord
    assert_equal 10, "\n".ord
  end

  # to_sym round-trips through Symbol.
  def test_to_sym_round_trips_through
    assert_equal :str, "str".to_sym
    assert_equal :"with space", "with space".to_sym
    assert_equal :"", "".to_sym
    assert_equal :a?, "a?".to_sym
    assert_equal true, ("x".to_sym == :x)
    assert_equal :abc, :abc.to_s.to_sym
  end

  # inspect escapes quotes, backslashes, named control chars and #{ #$ #@.
  def test_inspect_escapes_quotes_backslashes_named
    assert_equal "\"tab\\there\"", "tab\there".inspect
    assert_equal "\"quote\\\"d\"", "quote\"d".inspect
    assert_equal "\"new\\nline\"", "new\nline".inspect
    assert_equal "\"back\\\\slash\"", "back\\slash".inspect
    assert_equal "\"\\a\\b\\v\\f\\r\\e\"", "\a\b\v\f\r\e".inspect
    assert_equal "\"'single'\"", "'single'".inspect
    assert_equal "\"\"", "".inspect
    assert_equal "\"\\\#{x} \\\#$y \\\#@z #x # #\"", "\#{x} \#$y \#@z #x # #".inspect
    assert_equal "\"é日本😀\"", "é日本😀".inspect
    assert_equal "\" x\"", "\u00a0x".inspect
    assert_equal "\"​\"", "\u200b".inspect
    assert_equal 11, "a\\b\"c'd".inspect.size
    assert_equal 4, "\n".inspect.size
    assert_equal "[\"a\\tb\", \"c\\\"d\"]", ["a\tb", "c\"d"].inspect
  end

  # inspect output is itself a valid literal that reads back the same.
  def test_inspect_output_is_itself_a
    src = "x\ty\n\"z\""
    assert_equal "\"x\\ty\\n\\\"z\\\"\"", src.inspect
    assert_equal true, ("x\ty\n\"z\"" == src)
  end
end

class StringLiteralsTest < Minitest::Test
  # Single quotes keep #{} and \n literally; only \' and \\ are escapes.
  def test_single_quotes_keep_and_n
    assert_equal "single \#{not} \\n raw", 'single #{not} \n raw'
    assert_equal "'q' \\ \\n", '\'q\' \\ \n'
    assert_equal "", ''
  end

  # Double-quoted escapes, including \u{} with several codepoints and octal/hex.
  def test_double_quoted_escapes_including_u
    assert_equal "tab\tnl\\n", "tab\tnl\\n"
    assert_equal "é😀A\e |", "é\u{1F600}\x41\e\s|"
    assert_equal "AAB", "\101\u{41 42}"
    assert_equal 1, "\0".size
    assert_equal 1, "\cA".ord
  end

  # Percent literals with every delimiter pair, nesting included.
  def test_percent_literals_with_every_delimiter
    assert_equal "paren (nested) 'q'", %q(paren (nested) 'q')
    assert_equal "brack 3", %Q[brack #{1 + 2}]
    assert_equal "par (x)", %(par (x))
    assert_equal "ang", %<ang>
    assert_equal "bar", %|bar|
    assert_equal "br {n} 1", %Q{br {n} #{1}}
    assert_equal ["a", "b", "c"], %w[a b c]
    assert_equal [:a, :b, :c], %i[a b c]
    assert_equal [], %w[]
  end

  # Character literals are one-character strings.
  def test_character_literals_are_one_character
    assert_equal "a", ?a
    assert_equal "\n", ?\n
    assert_equal "é", ?é
    assert_equal " ", ?\s
    assert_equal "\t", ?\t
    assert_equal "ab", (?a + ?b)
    assert_equal "a)b", %q(a\)b)
    assert_equal 1, "\C-a".ord
    assert_equal 1, "\c?".bytesize
  end

  # Adjacent plain literals and backslash-newline continue one string.
  def test_adjacent_plain_literals_and_backslash
    assert_equal "adjacentlits", "adj" "acent" 'lits'
    assert_equal "line one continued", "line one \
continued"
  end

  # Heredocs: squiggly strips indentation, dash keeps it, quoted is raw.
  def test_heredocs_squiggly_strips_indentation_dash
    n = 5
    s = "str"
    h = <<~EOS
  indented #{n}
    more
  done
EOS
    assert_equal "indented 5\n  more\ndone\n", h
    r = <<~'RAW'
  raw #{n} \t
RAW
    assert_equal "raw \#{n} \\t\n", r
    d = <<-DASH
    keep #{s}
    DASH
    assert_equal "    keep str\n", d
    assert_equal "SHOUT IT", <<~EOS.strip.upcase
  shout #{"it"}
EOS
  end

  # Squiggly dedent skips blank lines, counts a tab, and keeps an interpolated value's own spaces.
  def test_squiggly_dedent_skips_blank_lines
    val = "  val"
    dedent = <<~EOS

  a

    b
	tab
    #{val}
EOS
    assert_equal "\na\n\n  b\n\ttab\n    val\n", dedent
    m = <<~EOS
  x #{[1, 2].map { |i|
    i * 2
  }.join(",")} y
EOS
    assert_equal "x 2,4 y\n", m
    a, b = <<~A, <<~B
  first
A
  second
B
    assert_equal "first\n", a
    assert_equal "second\n", b
  end

  # Interpolation calls to_s on each type; nil (typed or optional) is empty.
  def test_interpolation_calls_to_s_on
    n = 5
    s = "str"
    f = 2.5
    sym = :sy
    nl = nil #: Integer?
    some = 7 #: Integer?
    t = true
    arr = [1, 2] #: Array[Integer]
    assert_equal "5 2.5 str sy [] [7] true [1, 2]  false", "#{n} #{f} #{s} #{sym} [#{nl}] [#{some}] #{t} #{arr} #{nil} #{false}"
    assert_equal "Pt(3) and big", "#{Pt.new(3)} and #{n > 3 ? "big" : "small"}"
    assert_equal "nested inner 6", "nested #{"inner #{n + 1}"}"
    assert_equal "1", "#{"#{"#{1}"}"}"
    assert_equal "STR3", "#{s.upcase}#{s.size}"
    assert_equal "[2, 4]", "#{[1, 2].map { |x| x * 2 }}"
    assert_equal "pos", "#{if n > 0 then "pos" else "neg" end}"
    assert_equal "few", "#{case n when 1 then "one" when 4, 5 then "few" else "many" end}"
    assert_equal ["a5", "b", "strc"], %W[a#{n} b #{s}c]
    assert_equal 1, %W[#{n}].size
    maybe = "abc"[5]
    assert_equal "maybe=[] [b]", "maybe=[#{maybe}] [#{"abc"[1]}]"
    assert_equal "-0.0 1.0e+20 1.0e-05 100.0 0.3333333333333333 4611686018427387904 -5", "#{-0.0} #{1e20} #{1.0e-5} #{100.0} #{1.0 / 3} #{2**62} #{-5}"
    assert_equal "sym=a b arr=[:a, \"b\", 1.5, nil] h={\"k\" => :v} hs={a: 1}", "sym=#{:"a b"} arr=#{[:a, "b", 1.5, nil]} h=#{{ "k" => :v }} hs=#{{ a: 1 }}"
    u = 1 #: untyped
    assert_equal "u=1", "u=#{u}"
    assert_equal "cls=String tup=[1, \"a\"] tri=[1, \"a\", :b]", "cls=#{String} tup=#{[1, "a"]} tri=#{[1, "a", :b]}"
    assert_equal "only str", "only #{s}"
    assert_equal "str", "#{s}"
    assert_equal "strstr", "#{s}#{s}"
    assert_equal "5", "#{n}"
  end

  # Interpolation builds a fresh String each time.
  def test_interpolation_builds_a_fresh_string
    parts = [] #: Array[String]
    3.times { |i| parts << "p#{i}" }
    assert_equal ["p0", "p1", "p2"], parts
    assert_equal true, "a#{1}b" == "a1b"
  end
end

class StringMidTest < Minitest::Test
  def test_section_0
    assert_equal false, (MidPt.new(3).inspect == MidPt.new(3).to_s)
    assert_equal true, MidPt.new(3).inspect.include?("@x=3")
    assert_equal "[item]", Item.new.tag
    assert_equal "ITEM", Item.new.loud
    assert_equal "<#<Item>>", Item.new.shown
    assert_equal "abxyxyx", "ab".ljust(7, "xy")
    assert_equal "-=-=ab", "ab".rjust(6, "-=")
    assert_equal "*+*ab*+*+", "ab".center(9, "*+")
    assert_equal "éüü", "é".ljust(3, "ü")
    assert_equal "he wrd", "hello world".delete("lo")
    assert_equal "llo", "hello".delete("a-k")
    assert_equal "ll", "hello".delete("^l")
    e = assert_raises(ArgumentError) { "x".ljust(3, "") }
    assert_equal "zero width padding", e.message
    assert_equal "03.14|ab  |ff|FF|10|101|0xff|0b101|+3| 3|1.234568e+04|1.200000E-04|1.23457e+06|100000|0.0001|A|h|%|nil|sym", format("%05.2f|%-4s|%x|%X|%o|%b|%#x|%#b|%+d|% d|%e|%E|%g|%g|%g|%c|%c|%%|%p|%s", 3.14159, "ab", 255, 255, 8, 5, 255, 5, 3, 3, 12345.678, 0.00012, 1234567.0, 100000.0, 0.0001, 65, "hey", nil, :sym)
    assert_equal "hi        |     there|ab| 99.5%", format("%-10s|%10s|%.2s|%5.1f%%", "hi", "there", "abc", 99.5)
    assert_equal "x and 002.2", format("%<a>s and %<b>05.1f", a: "x", b: 2.25)
    assert_equal "1-", format("%{a}-%{b}", a: 1, b: nil)
    assert_equal "    1|2   |", format("%*d|%-*d|", 5, 1, 4, 2)
    assert_equal "3", format("%d", 3.99)
    assert_equal "-3", format("%d", -3.5)
    assert_equal "1.000000", format("%f", 1)
    assert_equal "0 2 2", format("%.0f %.0f %.0f", 0.5, 1.5, 2.5)
    assert_equal "[1, \"a\"]", format("%s", [1, "a"])
    assert_equal " -5|-5 |-05", format("%3d|%-3d|%03d", -5, -5, -5)
    assert_equal "3.14|    3.1416|1.23e+03  |", format("%.3g|%10.4f|%-10.2e|", 3.14159, Math::PI, 1234.5)
    assert_equal "Inf -Inf NaN", format("%f %f %e", Float::INFINITY, -Float::INFINITY, Float::NAN)
    assert_equal "1010 31", format("%B %d", 10, "0x1f")
    assert_equal "-003.142", sprintf("%08.3f", -3.14159)
    assert_equal "x and nil", "%s and %p" % ["x", nil]
    assert_equal "00042", "%05d" % 42
    assert_equal "12.3%", "%.1f%%" % 12.34
    assert_equal "ab   |", "%-5s|" % :ab
    e = assert_raises(ArgumentError) { format("%d %d", 1) }
    assert_equal "too few arguments", e.message
    e = assert_raises(ArgumentError) { format("%d", "abc") }
    assert_equal "invalid value for Integer(): \"abc\"", e.message
    ke = assert_raises(KeyError) { format("%<x>s", y: 1) }
    assert_equal "key<x> not found", ke.message
    assert_equal 5, "0b101".to_i(0)
    assert_equal 255, "ff".to_i(16)
    assert_equal 35, "z".to_i(36)
    assert_equal 12, "12abc".to_i(10)
    assert_equal 255, "ff".hex
    assert_equal -26, "-0x1A".hex
    assert_equal 511, "777".oct
    assert_equal 3, "0b11".oct
    assert_equal 0, "junk".hex
    assert_equal "ba", "az".succ
    assert_equal "aaa", "zz".succ
    assert_equal "b0", "a9".succ
    assert_equal "AAa", "Zz".succ
    assert_equal "2.0", "1.9".next
    assert_equal "", "".succ
    assert_equal 5, "hello world".count("lo")
    assert_equal 5, "hello".count("a-y")
    assert_equal 3, "hello".count("^l")
    assert_equal "abc", "aaabbbccc".squeeze
    assert_equal "abbbccc", "aaabbbccc".squeeze("a")
    assert_equal "misisipi", "mississippi".squeeze("sp")
    assert_equal "hELLO wORLD", "Hello World".swapcase
    assert_equal -1, "abc".casecmp("ABD")
    assert_equal true, "abc".casecmp?("ABC")
    assert_equal false, "abc".casecmp?("abd")
    assert_equal true, "hello".start_with?("x", "he")
    assert_equal true, "hello".end_with?("x", "lo")
    assert_equal "llo", "hello".delete_prefix("he")
    assert_equal "hel", "hello".delete_suffix("lo")
    assert_equal "hello", "hello".delete_prefix("x")
    assert_equal ["key", "=", "value=x"], "key=value=x".partition("=")
    assert_equal ["key=value", "=", "x"], "key=value=x".rpartition("=")
    assert_equal ["abc", "", ""], "abc".partition("z")
    assert_equal ["", "", "abc"], "abc".rpartition("z")
    assert_equal "ab", "abc".chop
    assert_equal "a", "a\r\n".chop
    assert_equal "a", "abc".chr
    assert_equal "", "".chr
    assert_equal false, "héllo".ascii_only?
    assert_equal true, "hello".ascii_only?
    assert_equal ["a", "b,c,d"], "a,b,c,d".split(",", 2)
    assert_equal ["a", "b  c"], "a b  c".split(" ", 2)
    assert_equal ["a", "b", "", ""], "a,b,,".split(",", -1)
    assert_equal ["a", "b", "c"], "abc".each_char.to_a
    assert_equal ["a\n", "b\n"], "a\nb\n".each_line.to_a
    each_lines = [] #: Array[String]
    "x\ny".each_line { |l| each_lines << l }
    assert_equal ["x\n", "y"], each_lines
    assert_equal "ello", "hello"[1..]
    assert_equal "ell", "hello".slice(1, 3)
    assert_equal "ababab", "ab" * 3
    assert_equal "STRASSE", "Straße".upcase
    assert_equal "àéî", "ÀÉÎ".downcase
    assert_equal "Hello world", "hello world".capitalize
    assert_equal "Hello", "HELLO".capitalize
    assert_equal "003.1", "%05.1f" % 3.14159
    assert_equal "ABC", "abc".tr("a-c", "A-C")
    assert_equal 294, "abc".bytes.sum
  end
end

class StringSplitTest < Minitest::Test
  # chars and each_char walk characters, not bytes.
  def test_chars_and_each_char_walk
    assert_equal ["h", "é", "y", "😀"], "héy😀".chars
    assert_equal [], "".chars
    assert_equal 2, "ab".chars.size
    out = [] #: Array[String]
    "añb".each_char { |c| out << c.upcase }
    assert_equal ["A", "Ñ", "B"], out
  end

  # each_char is an iterator: break and next are plain loop control.
  def test_each_char_is_an_iterator
    n = 0
    "stop here".each_char do |c|
      break if c == " "
      next if c == "o"

      n += 1
    end
    assert_equal 3, n
    assert_equal 0, vowels("xyz")
    assert_equal 1, vowels("xa")
  end

  # lines keeps the separator and never yields a trailing empty line.
  def test_lines_keeps_the_separator_and
    assert_equal ["a\n", "b\n"], "a\nb\n".lines
    assert_equal ["a\n", "b"], "a\nb".lines
    assert_equal [], "".lines
    assert_equal ["\n", "\n"], "\n\n".lines
    assert_equal ["a\r\n", "b"], "a\r\nb".lines
    assert_equal ["no newline"], "no newline".lines
    assert_equal 3, "x\n\ny".lines.size
  end

  # split with no separator (or nil) splits on runs of ASCII whitespace.
  def test_split_with_no_separator_or
    assert_equal ["one", "two", "three"], "one two  three".split
    assert_equal ["lead", "and", "trail"], "  lead and trail  ".split
    assert_equal ["tabs", "and", "newlines"], "tabs\tand\nnewlines".split
    assert_equal [], "".split
    assert_equal [], "   ".split
    assert_equal ["a", "b"], "a b".split(nil)
    assert_equal ["solo"], "solo".split
  end

  # split with a string drops trailing empties but keeps leading and inner ones.
  def test_split_with_a_string_drops
    assert_equal ["a", "b", "", "c"], "a,b,,c,,".split(",")
    assert_equal ["", "a"], ",a".split(",")
    assert_equal [], "".split(",")
    assert_equal [], ",,,".split(",")
    assert_equal ["a", "b"], "a--b--".split("--")
    assert_equal ["abc"], "abc".split("x")
    assert_equal 2, "a,b".split(",").size
  end

  # split("") yields characters.
  def test_split_yields_characters
    assert_equal ["a", "b", "c"], "abc".split("")
    assert_equal [], "".split("")
    assert_equal ["h", "é", "l", "l", "o"], "héllo".split("")
  end

  # Round trips through Array methods.
  def test_round_trips_through_array_methods
    assert_equal [1, 2, 3], "1,2,3".split(",").map(&:to_i)
    assert_equal "c+b+a", "a-b-c".split("-").reverse.join("+")
    assert_equal "cba", "abc".chars.reverse.join
    assert_equal "The Quick Brown", "the quick brown".split.map(&:capitalize).join(" ")
    assert_equal "v", "k=v".split("=")[1]
    assert_nil "k=".split("=")[1]
  end
end

class StringSymbolTest < Minitest::Test
  # Decision 23: Symbol is its own type, never equal to the String of the same name.
  def test_decision_23_symbol_is_its
    a = :apple
    assert_equal -1, (a <=> :banana)
    assert_equal 0, (a <=> :apple)
    assert_equal 1, (:b <=> :a)
    assert_equal -1, (:a <=> :ab)
    assert_equal true, (a == :apple)
    assert_equal false, (a == "apple")
    assert_equal true, (a != :x)
    assert_equal false, ("apple" == a)
    assert_equal true, (a.to_s == "apple")
    assert_equal true, (a == "apple".to_sym)
    assert_equal "apple", a.to_s
    assert_equal "apple", a.name
    assert_equal :apple, a.to_sym
    assert_equal :apple, a
    assert_equal 5, a.size
    assert_equal 5, :"héllo".size
    assert_equal 0, :"".size
    assert_equal true, (a.hash == :apple.hash)
    assert_equal false, (a.hash == :pear.hash)
  end

  # Comparable on Symbol
  def test_comparable_on_symbol
    a = :apple
    assert_equal true, (a < :b)
    assert_equal true, (a >= :apple)
    assert_equal true, (:z > :a)
    assert_equal true, (:a <= :a)
    assert_equal true, a.between?(:a, :b)
    assert_equal :m, :zz.clamp(:a, :m)
    assert_equal :b, :b.clamp(:a, :m)
  end

  # Symbol#inspect quotes only names that would not read back bare.
  def test_symbol_inspect_quotes_only_names
    syms = [:abc, :"a b", :a?, :b!, :c=, :[], :[]=, :+, :-, :-@, :+@, :<=>, :==, :===, :=~] #: Array[Symbol]
    syms += [:!, :!=, :!~, :<<, :>>, :<=, :>=, :**, :%, :/, :*, :&, :|, :^, :~, :<, :>]
    syms += [:"foo-bar", :"9a", :"", :A, :Abc?, :_x, :"a?b", :"a==", :"?", :"=", :"[]?", :call, :"a\nb", :"q\"q"]
    assert_equal [":abc", ":\"a b\"", ":a?", ":b!", ":c=", ":[]", ":[]=", ":+", ":-", ":-@", ":+@", ":<=>", ":==", ":===", ":=~"], syms[0, 15].map(&:inspect)
    assert_equal [":!", ":!=", ":!~", ":<<", ":>>", ":<=", ":>=", ":**", ":%", ":/", ":*", ":&", ":|", ":^", ":~", ":<", ":>"], syms[15, 17].map(&:inspect)
    assert_equal [":\"foo-bar\"", ":\"9a\"", ":\"\"", ":A", ":Abc?", ":_x", ":\"a?b\"", ":\"a==\"", ":\"?\"", ":\"=\"", ":\"[]?\"", ":call", ":\"a\\nb\"", ":\"q\\\"q\""], syms[32, 14].map(&:inspect)
    assert_equal 46, syms.size
  end

  # to_s gives the bare name, whatever inspect does.
  def test_to_s_gives_the_bare
    assert_equal "a b", :"a b".to_s
    assert_equal "+", :+.to_s
    assert_equal "q\"q", :"q\"q".to_s
  end

  # Sorting, min/max and &:sym blocks.
  def test_sorting_min_max_and_sym
    assert_equal [:a, :b, :c], [:b, :a, :c].sort
    assert_equal :b, [:b, :a].max
    assert_equal :a, [:b, :a].min
    assert_equal ["x", "y"], [:x, :y].map(&:to_s)
    assert_equal [:p, :q], %w[p q].map(&:to_sym)
    assert_equal [2, 4], %i[up down].map(&:size)
  end

  # case/when matches symbols by identity of name.
  def test_case_when_matches_symbols_by
    got = [:pear, :apple, :fig].map do |fruit|
      case fruit
      when :pear then "pear"
      when :apple then "apple!"
      else "other #{fruit}"
      end
    end
    assert_equal ["pear", "apple!", "other fig"], got
  end

  # Symbols as hash keys; Hash#inspect prints them as labels (decision 34).
  def test_symbols_as_hash_keys_hash
    config = { port: 8080, host: "localhost" } #: Hash[Symbol, untyped]
    assert_equal 8080, config[:port]
    assert_equal "localhost", config[:host]
    assert_equal [:port, :host], config.keys
    assert_nil config[:nope]
    counts = {} #: Hash[Symbol, Integer]
    [:a, :b, :a].each { |k| counts[k] = (counts[k] || 0) + 1 }
    assert_equal "{a: 2, b: 1}", counts.inspect
    assert_equal true, counts.key?(:a)
    assert_equal "hi x", greet(name: "x")
    assert_equal "hi y", greet(:name => "y")
    assert_equal "hi z", greet({ name: "z" })
    assert_equal true, :sym.frozen?
    assert_equal false, :sym.nil?
    assert_equal "interp", "#{:interp}"
  end

  # Symbols inside tuples (sort keys), tally/group_by/min_by, and as Hash values.
  def test_symbols_inside_tuples_sort_keys
    pairs = [[:b, 1], [:a, 2], [:a, 1]]
    assert_equal [[:a, 1], [:a, 2], [:b, 1]], pairs.sort
    assert_equal "{b: 1, a: 2}", pairs.map(&:first).tally.inspect
    assert_equal [:b, 1], pairs.max
    sp = [["b", :x], ["a", :y]]
    assert_equal [["a", :y], ["b", :x]], sp.sort
    by_name = { "a" => :x, "b c" => :"y z" } #: Hash[String, Symbol]
    assert_equal "{\"a\" => :x, \"b c\" => :\"y z\"}", by_name.inspect
    assert_equal [:x, :"y z"], by_name.values
    ss = %i[b a c]
    assert_equal [:c, :b, :a], ss.sort.reverse
    assert_equal ["B", "A", "C"], ss.map(&:to_s).map(&:upcase)
    assert_equal :a, ss.min_by(&:to_s)
    assert_equal true, ss.include?(:a)
    assert_equal [:a, :b, :c], ss.sort_by { |x| x.to_s }
    assert_equal "{1 => [:b, :a, :c]}", ss.group_by(&:size).inspect
  end
end

class StringTransformTest < Minitest::Test
  # Case mapping on ASCII and on letters with a single-rune upper/lower form.
  def test_case_mapping_on_ascii_and
    assert_equal "HELLO, WORLD", "Hello, World".upcase
    assert_equal "hello, world", "Hello, World".downcase
    assert_equal "", "".upcase
    assert_equal "ÀÉÎÕÜ", "àéîõü".upcase
    assert_equal "àéîõü", "ÀÉÎÕÜ".downcase
    assert_equal "Ÿ", "ÿ".upcase
    assert_equal "σ", "Σ".downcase
    assert_equal "123 !?", "123 !?".upcase
    assert_equal "mixed 42", "MiXeD 42".downcase
  end

  # capitalize lowercases the rest; a non-letter first char stays as is.
  def test_capitalize_lowercases_the_rest_a
    assert_equal "Hello world", "hELLO wORLD".capitalize
    assert_equal "", "".capitalize
    assert_equal "123abc", "123abc".capitalize
    assert_equal " x", " x".capitalize
    assert_equal "École", "éCOLE".capitalize
    assert_equal "A", "a".capitalize
    assert_equal "Abc", "ABC".capitalize
  end

  # reverse works on characters, so multibyte text survives.
  def test_reverse_works_on_characters_so
    assert_equal "olléh", "héllo".reverse
    assert_equal "", "".reverse
    assert_equal "b😀a", "a😀b".reverse
    assert_equal "ab", "ab".reverse.reverse
  end

  # strip family: ASCII whitespace only in these inputs.
  def test_strip_family_ascii_whitespace_only
    assert_equal "padded", "  padded  ".strip
    assert_equal "x", " \t\n\v\f\rx \t\n\v\f\r".strip
    assert_equal "", "".strip
    assert_equal "", "   ".strip
    assert_equal "x \t\n\v\f\r", " \t\n\v\f\rx \t\n\v\f\r".lstrip
    assert_equal " \t\n\v\f\rx", " \t\n\v\f\rx \t\n\v\f\r".rstrip
    assert_equal "in side  ", "  in side  ".lstrip
    assert_equal "  in side", "  in side  ".rstrip
    assert_equal "x", "x".lstrip
  end

  # chomp removes exactly one trailing \n, \r\n or \r.
  def test_chomp_removes_exactly_one_trailing
    assert_equal "a", "a\n".chomp
    assert_equal "a", "a\r\n".chomp
    assert_equal "a", "a\r".chomp
    assert_equal "a\n", "a\n\n".chomp
    assert_equal "a\n", "a\n\r".chomp
    assert_equal "a\r", "a\r\r\n".chomp
    assert_equal "a", "a".chomp
    assert_equal "", "".chomp
    assert_equal "", "\n".chomp
  end

  # Justification pads by character count; odd padding goes to the right in center.
  def test_justification_pads_by_character_count
    assert_equal "  x  ", "x".center(5)
    assert_equal " ab  ", "ab".center(5)
    assert_equal " abc  ", "abc".center(6)
    assert_equal "  héllo  ", "héllo".center(9)
    assert_equal "abc", "abc".center(2)
    assert_equal "abc", "abc".center(-1)
    assert_equal "   ", "".center(3)
    assert_equal "abc", "abc".center(3)
    assert_equal "x  ", "x".ljust(3)
    assert_equal "é  ", "é".ljust(3)
    assert_equal "abc", "abc".ljust(0)
    assert_equal "abc", "abc".ljust(-2)
    assert_equal "  x", "x".rjust(3)
    assert_equal "  é", "é".rjust(3)
    assert_equal "abc", "abc".rjust(-5)
    assert_equal "  ", "".rjust(2)
    assert_equal "|id  |  7|", "|" + "id".ljust(4) + "|" + "7".rjust(3) + "|"
  end

  # sub replaces the first match of a literal pattern, gsub all of them.
  def test_sub_replaces_the_first_match
    assert_equal "heLlo", "hello".sub("l", "L")
    assert_equal "heLLo", "hello".gsub("l", "L")
    assert_equal "baa", "aaa".sub("a", "b")
    assert_equal "a-b-c", "a.b.c".gsub(".", "-")
    assert_equal "abc", "abc".gsub("x", "y")
    assert_equal "", "abc".gsub("abc", "")
    assert_equal "-a-b-c-", "abc".gsub("", "-")
    assert_equal "-abc", "abc".sub("", "-")
    assert_equal "x", "".gsub("", "x")
    assert_equal "héllo world", "héllo wörld".gsub("ö", "o")
    assert_equal "bb", "aaaa".gsub("aa", "b")
    assert_equal "a+b", "a*b".sub("*", "+")
  end

  # tr maps characters one to one; a short to-list repeats its last character.
  def test_tr_maps_characters_one_to
    assert_equal "hippo", "hello".tr("el", "ip")
    assert_equal "hxxxx", "hello".tr("elo", "x")
    assert_equal "hello", "héllo".tr("é", "e")
    assert_equal "hello", "hello".tr("xyz", "abc")
    assert_equal "", "".tr("a", "b")
    assert_equal "cabcab", "abcabc".tr("abc", "cab")
  end

  # Chained transforms keep each intermediate result immutable.
  def test_chained_transforms_keep_each_intermediate
    s = "  Mixed Case  "
    t = s.strip.downcase.tr(" ", "_")
    assert_equal "mixed_case", t
    assert_equal "  Mixed Case  ", s
    assert_equal "ABAB CD ", "ab".upcase * 2 + "cd".center(4).upcase
  end
end

class StringDynamicTest < Minitest::Test
  # Decision 23 at run time: a type switch tells String from Symbol although both are Go strings.
  def test_decision_23_at_run_time
    vals = ["str", :sym, 1, nil, "", :""] #: Array[untyped]
    kinds = vals.map do |v|
      case v
      when String then "String #{v.size}"
      when Symbol then "Symbol #{v.inspect}"
      when nil then "nil"
      else "other"
      end
    end
    assert_equal ["String 3", "Symbol :sym", "other", "nil", "String 0", "Symbol :\"\""], kinds
    tests = vals.map { |v| "#{v.is_a?(String)} #{v.is_a?(Symbol)} #{v.inspect}" }
    assert_equal ["true false \"str\"", "false true :sym", "false false 1", "false false nil", "true false \"\"", "false true :\"\""], tests
    assert_equal ["str", "sym", "1", "", "", ""], vals.map(&:to_s)
    assert_equal ["\"str\"", ":sym", "1", "nil", "\"\"", ":\"\""], vals.map(&:inspect)
  end

  # Untyped Hash keys: "a" and :a are different keys.
  def test_untyped_hash_keys_a_and
    h = {} #: Hash[untyped, Integer]
    h["a"] = 1
    h[:a] = 2
    h["a"] = 3
    assert_equal 2, h.size
    assert_equal "{\"a\" => 3, a: 2}", h.inspect
    assert_equal 3, h["a"]
    assert_equal 2, h[:a]
    assert_nil h["b"]
    assert_equal ["a", :a], ["a", :a, "a", :a].uniq
  end

  # Decision 32: String methods called on an untyped value.
  def test_decision_32_string_methods_called
    u = "Hello" #: untyped
    assert_equal "HELLO", u.upcase
    assert_equal 5, u.size
    assert_equal "e", u[1]
    assert_equal "Hello!", (u + "!")
    assert_equal ["He", "", "o"], u.split("l")
    assert_equal true, u.start_with?("He")
    assert_equal 2, u.index("l")
    assert_equal "\"Hello\"", u.inspect
    assert_equal true, u == "Hello"
    assert_equal true, u.equal?(u)
    assert_equal "  Hello  ", u.center(9)
    assert_equal :Hello, u.to_sym
    assert_equal 5, u.chars.size
    assert_equal "Hello!", "#{u}!"
    assert_equal "hello", u.to_s.downcase
    s = :sym #: untyped
    assert_equal "sym", s.to_s
    assert_equal ":sym", s.inspect
    assert_equal 3, s.size
    assert_equal true, s == :sym
    assert_equal false, (s == "sym")
    assert_equal :sym, s.to_sym
    assert_equal -1, (s <=> :syn)
    assert_equal "Hello sym", "#{u} #{s}"
  end

  # Arity and argument types are checked like MRI; unknown methods raise NoMethodError.
  def test_arity_and_argument_types_are
    u = "Hello" #: untyped
    e = assert_raises(ArgumentError) { u.reverse(1) }
    assert_equal "wrong number of arguments (given 1, expected 0)", e.message
    assert_raises(TypeError) { u + 1 } # message wording: dynamic_bug_type_error_message
    assert_raises(NoMethodError) { u.no_such_method }
  end

  # respond_to?, send and class on literal receivers.
  def test_respond_to_send_and_class
    assert_equal true, "abc".respond_to?(:upcase)
    assert_equal false, "abc".respond_to?(:foo)
    assert_equal true, :a.respond_to?(:size)
    assert_equal "ABC", "abc".send(:upcase)
    assert_equal "  abc  ", "abc".public_send(:center, 7)
    assert_equal "a", :a.send(:to_s)
    assert_equal "xy", "x".send(:+, "y")
    assert_equal String, "a".class
    assert_equal "String", "a".class.name
    assert_equal true, "a".is_a?(Comparable)
    assert_equal true, "a".is_a?(Object)
    assert_equal true, "a".is_a?(String)
    assert_equal false, "a".kind_of?(Symbol)
    d = ident(" Hello\n")
    assert_equal " Hello\n Hello\n", (d * 2)
    assert_equal false, d.between?("a", "z")
    assert_equal "A", d.clamp("A", "B")
    assert_equal "Hello", d.strip
    assert_equal " Hello", d.chomp
    assert_equal "Hello\n", d.lstrip
    assert_equal " Hello", d.rstrip
    assert_equal "\nolleH ", d.reverse
    assert_equal " hello\n", d.capitalize
    assert_equal " hello\n", d.downcase
    assert_equal :" Hello\n", d.to_sym
    assert_equal false, d.nil?
    assert_equal " Hello\n", d.dup
    assert_equal " ", d[0]
    assert_equal "\n", d[-1]
    assert_nil d[99]
    assert_nil d.index("z")
    assert_equal ["Hello"], d.split
    assert_equal ["Hello"], d.split(nil)
    assert_equal [" He", "", "o\n"], d.split("l")
    assert_equal " Hello\n  ", d.ljust(9)
    assert_equal "   Hello\n", d.rjust(9)
    assert_equal 7, d.length
    assert_equal 7, d.bytesize
    assert_equal false, d.empty?
    assert_equal [" Hello\n"], d.lines
    assert_equal 32, d.ord
    assert_equal true, d.include?("e")
    assert_equal true, d.end_with?("\n")
    assert_equal 7, d.chars.size
    assert_equal " HeLlo\n", d.sub("l", "L")
    assert_equal " HeLLo\n", d.gsub("l", "L")
    assert_equal " He001\n", d.tr("lo", "01")
    assert_equal "Hello!", "#{d.strip}!"
    assert_equal true, (d == " Hello\n")
    assert_equal false, (d != " Hello\n")
    assert_equal true, (" Hello\n" == d)
    num = ident("42")
    assert_equal 43, num.to_i + 1
    assert_equal 42.0, num.to_f
    assert_equal 84, (num.to_i * 2)
    ds = ident(:sy)
    assert_equal "sy", ds.to_s
    assert_equal ":sy", ds.inspect
    assert_equal 2, ds.size
    assert_equal "sy", ds.name
    assert_equal true, (ds == :sy)
    assert_equal false, (ds == "sy")
    assert_equal :sy, ds.to_sym
    assert_equal "sy", "#{ds}"
    dn = ident(nil)
    assert_equal "[]", "[#{dn}]"
    assert_equal "", dn.to_s
    assert_equal "nil", dn.inspect
    assert_raises(TypeError) { d.center("x") }
    assert_raises(TypeError) { d.split(1) }
    assert_raises(ArgumentError) { d.sub("a") }
  end
end

class StringBugSymbolInspectTest < Minitest::Test
  def test_section_0
    assert_equal ":é", :"é".inspect
    assert_equal ":@iv", :"@iv".inspect
    assert_equal ":@@cv", :"@@cv".inspect
    assert_equal ":$g", :"$g".inspect
    assert_equal ":$1", :"$1".inspect
    assert_equal ":`", :"`".inspect
    assert_equal ":\"a?=\"", :"a?=".inspect
    assert_equal ":A?", :"A?".inspect
    assert_equal ":A=", :"A=".inspect
    assert_equal ":\"a!=\"", :"a!=".inspect
    assert_equal ":\"@1\"", :"@1".inspect
    assert_equal ":\"@a?\"", :"@a?".inspect
    assert_equal ":\"@@\"", :"@@".inspect
    assert_equal ":@é", :"@é".inspect
    assert_equal ":\"$\"", :"$".inspect
    assert_equal ":$-w", :"$-w".inspect
    assert_equal ":\"$-\"", :"$-".inspect
    assert_equal ":\"$-ww\"", :"$-ww".inspect
    assert_equal ":$12", :"$12".inspect
    assert_equal ":\"$01\"", :"$01".inspect
    assert_equal ":$0", :"$0".inspect
    assert_equal ":\"$ab?\"", :"$ab?".inspect
    assert_equal ":$\\", :"$\\".inspect
    assert_equal ":$`", :"$`".inspect
    assert_equal ":$'", :"$'".inspect
    assert_equal ":\"$%\"", :"$%".inspect
    assert_equal ":\"~@\"", :"~@".inspect
    assert_equal ":\"!@\"", :"!@".inspect
    assert_equal ":\"&.\"", :"&.".inspect
    assert_equal ":\"=\"", :"=".inspect
  end
end

class StringBugsTest < Minitest::Test
  # adjacent literals and a line continuation concatenate
  def test_adjacent_literals_and_a_line
    n_interp = 1
    assert_equal "tu1v", "t" "u" "#{n_interp}" 'v'
    assert_equal "total: 1 items", "total: #{n_interp} " \
      "items"
  end

  # clamp with min > max raises
  def test_clamp_with_min_max_raises
    e = assert_raises(ArgumentError) { "b".clamp("c", "a") }
    assert_equal "min argument must be less than or equal to max argument", e.message
    foo_obj = Foo.new
    assert_equal true, foo_obj.to_s.start_with?("#<Foo:0x")
    assert_equal true, foo_obj.to_s.end_with?(">")
    assert_equal true, "#{foo_obj}".include?(":0x")
    assert_equal true, foo_obj.inspect.start_with?("#<Foo:0x")
    fmt = Fmt.new
    assert_equal "<s>|\"s\"", fmt.show("s")
    assert_equal "<12>|12", fmt.show(12)
    assert_equal "<2.5>|2.5", fmt.show(2.5)
    assert_equal "<sym>|:sym", fmt.show(:sym)
    assert_equal "<ab>|\"ab\"", fmt.show("a" + "b")
    assert_equal "<>|nil", fmt.show(nil)
  end

  # invalid UTF-8 bytes inspect as \xNN
  def test_invalid_utf_8_bytes_inspect
    assert_equal "\"\\xFF\"", "\xff".inspect
    assert_equal "\"a\\xE1b\"", "a\xe1b".inspect
  end

  # String#* with a negative count raises
  def test_string_with_a_negative_count
    e = assert_raises(ArgumentError) { "ab" * -1 }
    assert_equal "negative argument", e.message
  end

  # != compares by value, not identity
  def test_compares_by_value_not_identity
    ch = "abc".chars[1]
    assert_equal false, (ch != "b")
    built = "a" + "b"
    assert_equal false, (built != "ab")
    assert_equal false, ("x".upcase != "X")
    assert_equal false, ("ab".dup != "ab")
    ne_count = 0
    "abc".each_char { |c| ne_count += 1 if c != "b" }
    assert_equal 2, ne_count
    assert_equal false, ("b" != "abc"[1])
    assert_equal "String", kind("a")
    assert_equal "Symbol", kind(:a)
    assert_equal "a|\"a\"|true|false", desc("a")
    assert_equal "a|:a|false|true", desc(:a)
    assert_equal "ab|\"ab\"|true|false", desc("a" + "b")
    assert_equal "1|1|false|false", desc(1)
    objs = ["s", :t, 2.5] #: Array[Object]
    assert_equal ["s", "t", "2.5"], objs.map { |o| o.to_s }
    assert_equal ["s", :t, 2.5], objs
  end

  # ord on an empty string raises
  def test_ord_on_an_empty_string
    e = assert_raises(ArgumentError) { "".ord }
    assert_equal "empty string", e.message
  end

  # split(" ") is awk-style whitespace splitting
  def test_split_is_awk_style_whitespace
    assert_equal ["a", "b"], "a  b ".split(" ")
    assert_equal ["a", "b"], " a b".split(" ")
  end

  # awk-style split only breaks on ASCII whitespace
  def test_awk_style_split_only_breaks
    assert_equal ["a b　c", "d"], "a\u00a0b\u3000c d".split
    assert_equal 1, "x\u00a0y".split(nil).size
  end

  # sub/gsub replacement strings honor backreferences
  def test_sub_gsub_replacement_strings_honor
    assert_equal "abbc", "abc".sub("b", "\\0\\0")
    assert_equal "abb", "a'b".gsub("'", "\\'")
    assert_equal "x<->y", "x-y".gsub("-", "<\\&>")
    assert_equal "aac", "abc".gsub("b", "\\`")
    assert_equal "a[]c", "abc".sub("b", "[\\1]")
    assert_equal "\\", "a".sub("a", "\\\\")
  end

  # to_f parses the longest numeric prefix
  def test_to_f_parses_the_longest
    assert_equal 3.5, "3.5abc".to_f
    assert_equal 1.5, "1.5 kg".to_f
    assert_equal 1.0, "1e".to_f
    assert_equal 1.5, "1.5.3".to_f
    assert_equal 1.0, "1__0".to_f
    assert_equal 150.0, "  +1.5e2x".to_f
    assert_equal 0.0, "Infinity".to_f
    assert_equal 0.0, "NaN".to_f
    assert_equal 0.0, "inf".to_f
    assert_equal 0.0, "0x1p3".to_f
  end

  # to_i accepts underscores between digits
  def test_to_i_accepts_underscores_between
    assert_equal 1000, "1_000".to_i
    assert_equal 12345, "12_345xyz".to_i
  end

  # tr with a repeated from-char uses its first mapping
  def test_tr_with_a_repeated_from
    assert_equal "heyyo", "hello".tr("ll", "xy")
  end

  # tr with an empty to-set deletes
  def test_tr_with_an_empty_to
    assert_equal "heo", "hello".tr("l", "")
  end

  # tr ranges and ^ negation
  def test_tr_ranges_and_negation
    assert_equal "ifmmp", "hello".tr("a-y", "b-z")
    assert_equal "**ll*", "hello".tr("^l", "*")
  end

  # strip/to_i/to_f do not treat Unicode spaces as whitespace
  def test_strip_to_i_to_f
    assert_equal " x ", "\u00a0x\u00a0".strip
    assert_equal "　x", "\u3000x".strip
    assert_equal 0, "\u00a012".to_i
    assert_equal 0, "\u300012".to_i
    assert_equal 0.0, "\u00a01.5".to_f
    assert_equal String, ident_class("a").class
    assert_equal Symbol, ident_class(:a).class
    s_cmp = ident_cmp("b")
    assert_equal 1, (s_cmp <=> "a")
    assert_equal 0, (s_cmp <=> "b")
    y_cmp = ident_cmp(:b)
    assert_equal -1, (y_cmp <=> :c)
  end

  # case mapping follows Unicode special casing
  def test_case_mapping_follows_unicode_special
    assert_equal "STRASSE", "straße".upcase
    assert_equal "Ss", "ß".capitalize
    assert_equal "i̇", "İ".downcase
    assert_equal "FF", "ﬀ".upcase
    assert_equal "ǅa", "ǆa".capitalize
  end

  # Non-printable non-ASCII is \uXXXX (\u{X} above U+FFFF); format (Cf) and private-use characters print raw, like MRI.
  def test_non_printable_non_ascii_is
    assert_equal "\"\\u0085|\\u2028|\\u2029| |­|​|\"", "\u0085| | | |­|​|".inspect
    assert_equal "\"\\u{10FFFF}|󠀁|😀|�|é\"", "\u{10FFFF}|\u{E0001}|\u{1F600}|�|é".inspect
  end

  # Invalid bytes are \xNN, one per byte; a surrogate encoding is invalid too.
  def test_invalid_bytes_are_xnn_one
    assert_equal "\"\\xED\\xA0\\x80|\\xE1\\x80|\\xC3\"", "\xed\xa0\x80|\xe1\x80|\xc3".inspect
  end

  # Named escapes and the #{ #$ #@ guard still apply around them.
  def test_named_escapes_and_the_guard
    assert_equal "\"\\e\\a\\b\\f\\v\\u0085\\\#{x}\\\#$y\\\#@z#a\"", "\e\a\b\f\v\u0085\#{x}\#$y\#@z#a".inspect
    assert_equal "[\"\\u2028\", \"\\xFF\"]", [" ", "\xff"].inspect
  end

  # ASCII-only symbols are US-ASCII in MRI, so controls stay \xNN.
  def test_ascii_only_symbols_are_us
    assert_equal ":\"a\\x00b\"", :"a\x00b".inspect
    assert_equal "{\"a\\x7F\": 1}", {"a\x7f": 1}.inspect
  end
end

class StringNilReceiverTest < Minitest::Test
  # Decision 20: calling a method on a nil String? raises NoMethodError, like MRI.
  def test_decision_20_calling_a_method
    s = "abc"
    c = s[5]
    assert_raises(NoMethodError) { c.upcase }
  end

  # Narrowing makes the call safe; to_s/inspect/nil? are fine on nil itself.
  def test_narrowing_makes_the_call_safe
    s = "abc"
    c = s[5]
    assert_equal true, c.nil?
    assert_equal "", c.to_s
    assert_equal "nil", c.inspect
    assert_equal true, (c == nil)
    d = s[1]
    up = d ? d.upcase : "unreached"
    assert_equal "B", up
    e = s[-1] || "fallback"
    assert_equal "c", e
    assert_equal "fallback", (s[7] || "fallback")
    x = s.index("q")
    assert_equal -1, x ? x + 1 : -1
    assert_equal "A", label(s[0])
    assert_equal "none", label(s[3])
  end
end
