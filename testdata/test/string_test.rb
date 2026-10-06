# rbs_inline: enabled

require "minitest/autorun"

# Helpers for the checks that were testdata/run/string_split.rb.
#: (String) -> Integer
def vowels(s)
  s.each_char { |c| return 1 if "aeiou".include?(c) }
  0
end

# Helpers for the checks that were testdata/run/string_symbol.rb.
#: (Hash[Symbol, String]) -> String
def string_greet(opts) = "hi #{opts[:name]}"

# Helpers for the checks that were testdata/run/string_dynamic.rb.
#: (untyped) -> untyped
def string_ident(v) = v

#: (Object) -> String
def string_kind(o) = o.class.name

#: (Object) -> String
def desc(o) = "#{o}|#{o.inspect}|#{o.is_a?(String)}|#{o.is_a?(Symbol)}"

#: (untyped) -> untyped
def string_ident_class(v) = v

#: (untyped) -> untyped
def string_ident_cmp(v) = v

# Helpers for the checks that were testdata/run/string_nil_receiver.rb.
#: (String?) -> String
def string_label(v)
  return "none" unless v

  v.capitalize
end

module StringTests
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

  # Helpers for the checks that were testdata/run/string_bugs.rb.
  # Kernel#to_s/inspect show a heap address
  class Foo
  end

  # a literal passed to a generic param keeps String
  class Fmt
    # @rbs [T] (T) -> String
    def show(x) = "<#{x}>|#{x.inspect}"
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
      assert_equal "hi x", string_greet(name: "x")
      assert_equal "hi y", string_greet(:name => "y")
      assert_equal "hi z", string_greet({ name: "z" })
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
      d = string_ident(" Hello\n")
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
      num = string_ident("42")
      assert_equal 43, num.to_i + 1
      assert_equal 42.0, num.to_f
      assert_equal 84, (num.to_i * 2)
      ds = string_ident(:sy)
      assert_equal "sy", ds.to_s
      assert_equal ":sy", ds.inspect
      assert_equal 2, ds.size
      assert_equal "sy", ds.name
      assert_equal true, (ds == :sy)
      assert_equal false, (ds == "sy")
      assert_equal :sy, ds.to_sym
      assert_equal "sy", "#{ds}"
      dn = string_ident(nil)
      assert_equal "[]", "[#{dn}]"
      assert_equal "", dn.to_s
      assert_equal "nil", dn.inspect
      assert_raises(TypeError) { d.center("x") }
      err = begin # rb2go's dynamic argument check (decision 20) words this TypeError differently from MRI's Regexp check
        d.split(1)
        nil
      rescue TypeError => e
        e
      end
      assert_equal false, err.nil?
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
      assert_equal true, foo_obj.to_s.start_with?("#<StringTests::Foo:0x")
      assert_equal true, foo_obj.to_s.end_with?(">")
      assert_equal true, "#{foo_obj}".include?(":0x")
      assert_equal true, foo_obj.inspect.start_with?("#<StringTests::Foo:0x")
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
      assert_equal "String", string_kind("a")
      assert_equal "Symbol", string_kind(:a)
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
      assert_equal String, string_ident_class("a").class
      assert_equal Symbol, string_ident_class(:a).class
      s_cmp = string_ident_cmp("b")
      assert_equal 1, (s_cmp <=> "a")
      assert_equal 0, (s_cmp <=> "b")
      y_cmp = string_ident_cmp(:b)
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
      assert_equal "\"\\u0001\\u007F\\e\"", "\x01\x7f\e".inspect # a UTF-8 string's controls; a Symbol's are \x00
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
      assert_equal "A", string_label(s[0])
      assert_equal "none", string_label(s[3])
    end
  end

  # ruby/spec core/symbol gaps (#49)
  class StringRubySpecSymbolTest < Minitest::Test
    def test_symbol_string_methods
      assert_equal [2, true, false], [:ab.length, :"".empty?, :a.empty?]
      assert_equal %i[AB ab Ab aB b ba], [:ab.upcase, :AB.downcase, :ab.capitalize, :Ab.swapcase, :a.succ, :az.next]
      assert_equal [0, 1, true], [:a.casecmp(:A), :b.casecmp(:a), :a.casecmp?(:A)]
      assert_equal [true, false, true], [:abc.start_with?("a"), :abc.start_with?("b"), :abc.end_with?("bc")]
      assert_equal ["a", "b", "bc", "ab"], [:a.id2name, :abc[1], :abc[1, 2], :abc[0..1]]
      assert_nil :abc[5]
    end
  end

  # ruby/spec core/string, core/matchdata and core/regexp gaps (#49)
  class StringRubySpecTest < Minitest::Test
    def test_byte_methods
      assert_equal [3, nil, 3, 4], ["abcb".rindex("b"), "abc".rindex("z"), "héllo".byteindex("l"), "héllo".byterindex("l")]
      assert_equal ["é", nil, "", "c"], ["héllo".byteslice(1, 2), "abc".byteslice(3), "abc".byteslice(3, 1), "abc".byteslice(-1)]
      assert_equal [195, nil, 195], ["é".getbyte(0), "é".getbyte(5), "ab".sum]
      assert_equal [104, 233], "hé".codepoints
      bytes = [] #: Array[Integer]
      "hé".each_byte { |b| bytes << b }
      "hé".each_codepoint { |c| bytes << c }
      assert_equal [104, 195, 169, 104, 233], bytes
    end

    def test_upto
      out = [] #: Array[String]
      "a".upto("e") { |s| out << s }
      "9".upto("11") { |s| out << s }
      "09".upto("11") { |s| out << s }
      "az".upto("bc") { |s| out << s }
      "b".upto("a") { |s| out << s }
      assert_equal %w[a b c d e 9 10 11 09 10 11 az ba bb bc], out
    end

    def test_misc
      assert_equal [:a, "ab", true, false], ["a".intern, "ab".b, "a".eql?("a"), "a".eql?(:a)]
      assert_equal "\"a\\\"\\\\\\n\\e\\\#{x}\\u00E9\\u{1F600}\\x00\"", "a\"\\\n\e\#{x}é😀\x00".dump
    end

    def test_matchdata
      m = "héllo world".match(/(?<first>l+)(?<o>o)? (?<w>w)(?<x>x)?/)
      refute_nil m
      return unless m
      assert_equal [5, 5, "héllo world"], [m.size, m.length, m.string]
      assert_equal %w[first o w x], m.names
      assert_equal({ "first" => "ll", "o" => "o", "w" => "w", "x" => nil }, m.named_captures)
      assert_equal ["ll", "o", nil, nil], m.values_at(1, 2, 4, 7)
      assert_equal ["ll", "w", "ll", 2, nil], [m[:first], m["w"], m.match(1), m.match_length(1), m.match_length(4)]
      assert_equal [2, 7, nil], [m.begin(0), m.end(0), m.begin(4)]
      assert_equal [2, 4], m.offset(1)
      assert_equal [nil, nil], m.offset(4)
      assert_equal [3, 8], m.byteoffset(0)
      assert_raises(IndexError) { m.begin(9) }
      assert_raises(IndexError) { m.match(9) }
    end

    def test_regexp_names
      assert_equal %w[a b], /(?<a>x)(?<b>y)(?<a>z)/.names
      assert_equal({ "a" => [1], "b" => [2] }, /(?<a>x)(?<b>y)/.named_captures)
      assert_equal [], /a/.names
      assert_equal "a\\.b\\*c", Regexp.quote("a.b*c")
    end
  end

  # ruby/spec language gaps (#49): /x regexps and named captures assigned to locals
  STRING_DATE_RE = /
    (\d{4}) - # year
    (\d{2}) - # month
    (\d{2})   # day
  /x

  class StringRubySpecRegexpSyntaxTest < Minitest::Test
    def test_extended_regexp
      m = "on 2024-03-09 ok".match(STRING_DATE_RE)
      assert_equal ["2024", "09"], [m&.[](1), m&.[](3)]
      assert_equal [true, 2], [STRING_DATE_RE.source.include?("year"), STRING_DATE_RE.options]
      assert_equal [0, 0, 0, 0], ["a b" =~ /a\ b/x, "a#b" =~ /a\#b/x, "a b" =~ /a[ ]b/x, "ab" =~ /a b/xi]
    end

    # #52: an interpolated /x regexp strips the values' spacing too; an embedded
    # Regexp's (?-mix:...) keeps its own, as does a (?-x) group
    def test_extended_interpolated
      w = "a b"
      r = /x y/
      assert_equal [0, nil, "/a b/x"], ["ab1" =~ /#{w} \d # digit
      /x, "a b1" =~ /#{w} \d/x, /#{w}/x.inspect]
      assert_equal [nil, 0, "(?-mix:x y) z"], ["x y z" =~ /#{r} z/x, "x yz" =~ /#{r} z/x, /#{r} z/x.source]
      assert_equal [0, 0, 0], ["a b" =~ /(?-x:a b)|#{w}/x, "a b" =~ /(?-x)a b/x, "ab c" =~ /a b(?-x) c/x]
    end

    def test_named_capture_locals
      line = "user=ann id=42"
      found = /user=(?<name>\w+) id=(?<id>\d+)/ =~ line
      assert_equal [0, "ann", "42"], [found, name, id]
      pos = /(?<word>z+)/ =~ "abc"
      assert_nil pos
      assert_nil word
    end
  end

  # ruby/spec core/string gaps (#49): unpack and unpack1
  class StringRubySpecUnpackTest < Minitest::Test
    def test_unpack_integers
      assert_equal [[97, 98, 99], [97, 98], [-1, 1], [97, 98, nil]], ["abc".unpack("C*"), "abc".unpack("c2"), "\xff\x01".b.unpack("c*"), "ab".unpack("C3")]
      assert_equal [[513, 1027], [258], [513], [16909060], [67305985], [16909060], [-1]],
                   ["\x01\x02\x03\x04".unpack("S*"), "\x01\x02".unpack("n"), "\x01\x02".unpack("v"), "\x01\x02\x03\x04".unpack("N"), "\x01\x02\x03\x04".unpack("V"), "\x01\x02\x03\x04".unpack("L>"), "\xff\xff".b.unpack("s")]
      assert_equal [[1], [-1]], ["\x01\x00\x00\x00\x00\x00\x00\x00".unpack("Q"), "\xff\xff\xff\xff\xff\xff\xff\xff".b.unpack("q")]
    end

    def test_unpack_strings
      assert_equal [["hello", "world"], ["hi"], ["hi", "there"], ["ab", "c"], [104, 233]],
                   ["hello world".unpack("a5 x a*"), "hi  \0".unpack("A*"), "hi\0there".unpack("Z*a*"), "abc".unpack("a2a"), "hé".unpack("U*")]
      assert_equal [["4142"], ["1424"], ["0100000101000010"], ["1000001001000010"], ["414"]],
                   ["AB".unpack("H*"), "AB".unpack("h*"), "AB".unpack("B*"), "AB".unpack("b*"), "AB".unpack("H3")]
      assert_equal [["hello"], "hello", 97, nil], ["aGVsbG8=".unpack("m"), "aGVsbG8=\n".unpack1("m"), "abc".unpack1("C"), "".unpack1("C")]
    end
  end

  # Array#pack and the rest of String#unpack (#54, decision 138); binary results compare as bytes since rb2go's inspect has no binary encoding.
  class StringPackTest < Minitest::Test
    LONG = "hello world, this is long enough to wrap beyond forty five bytes!!" #: String

    def test_pack_strings
      assert_equal ["a", "ab\x00", "ab ", "ab\x00", "ab", "ab", "", "\x00"].map(&:bytes),
                   [["ab"].pack("a"), ["ab"].pack("a3"), ["ab"].pack("A3"), ["ab"].pack("Z*"), ["ab"].pack("Z2"), ["ab"].pack("A*"), ["ab"].pack("a0"), [nil].pack("a")].map(&:bytes)
      assert_equal [[0xb3, 0x20], [0xcd, 0], [0xb3, 0], [0x80], [0xa1, 0xf0, 0], [0x1a, 0x0f], [0x12, 0x30], [0x60]],
                   [["10110011001"].pack("B*"), ["10110011001"].pack("b10"), ["10110011001"].pack("B9"), ["1"].pack("B"),
                    ["a1F"].pack("H5"), ["a1F"].pack("h*"), ["xyz"].pack("H*"), ["2a3"].pack("B*")].map(&:bytes)
      assert_equal [[104, 195, 169, 240, 159, 152, 128], [0, 1, 127, 0x81, 0, 0xff, 0x7f, 0x81, 0x80, 0, 0xa0, 0x80, 0x80, 0x80, 0x80, 0]],
                   [[104, 233, 0x1F600].pack("U*").bytes, [0, 1, 127, 128, 16383, 16384, 2**40].pack("w*").bytes]
      assert_equal [237, 160, 128, 244, 144, 128, 128, 253, 191, 191, 191, 191, 191], [0xD800, 0x110000, 0x7fffffff].pack("U*").bytes # MRI's rb_uv_to_utf8
    end

    def test_pack_encodings
      assert_equal ["M:&5L;&\\@=V]R;&0L('1H:7,@:7,@;&]N9R!E;F]U9V@@=&\\@=W)A<\"!B97EO\n5;F0@9F]R='D@9FEV92!B>71E<R$A\n", "#:&5L\n", ""],
                   [[LONG].pack("u"), ["hello"].pack("u3")[0, 6], [""].pack("u")]
      assert_equal ["aGVsbG8gd29ybGQsIHRoaXMgaXMgbG9uZyBlbm91Z2ggdG8gd3JhcCBiZXlv\nbmQgZm9ydHkgZml2ZSBieXRlcyEh\n", "aGk=", "aGVs\nbG8=\n"],
                   [[LONG].pack("m"), ["hi"].pack("m0"), ["hello"].pack("m3")]
      assert_equal ["h=C3=A9llo =3D x =\n\nnext=\n", "h=C3=\n=A9l=\nlo=\n", "12=\n"], [["héllo = x \nnext"].pack("M"), ["héllo"].pack("M3"), [12].pack("M")]
      assert_equal [LONG, LONG, LONG, "héllo = x \nnext".bytes], [[LONG].pack("u").unpack1("u"), [LONG].pack("m").unpack1("m"), [LONG].pack("m0").unpack1("m0"), ["héllo = x \nnext"].pack("M").unpack1("M").bytes]
    end

    def test_pack_integers
      q = [1, 0, 0, 0, 0, 0, 0, 0, 254, 255, 255, 255, 255, 255, 255, 255, 44, 1, 0, 0, 0, 0, 0, 0]
      l = [1, 0, 0, 0, 254, 255, 255, 255, 44, 1, 0, 0]
      want = [[1, 254, 44], [1, 254, 44], [1, 0, 254, 255, 44, 1], [0, 1, 255, 254, 1, 44], l, [0, 0, 0, 1, 255, 255, 255, 254, 0, 0, 1, 44],
              [0, 1, 255, 254, 1, 44], [1, 0, 254, 255, 44, 1], l, [0, 0, 0, 0, 0, 0, 0, 1, 255, 255, 255, 255, 255, 255, 255, 254, 0, 0, 0, 0, 0, 0, 1, 44],
              q, q, q, l, l, q, q, [1, 0, 254, 255, 44, 1], q]
      assert_equal want, %w[C* c* S* s>* L* N* n* v* V* Q>* q* J* j<* I* i!* L_* l!* S!* Q!*].map { |f| [1, -2, 300].pack(f).bytes }
      assert_equal [[1, 254, 44], [0, 1, 255, 254, 1, 44], [1, 0, 0, 0, 0, 0, 0, 0]], [[1, -2, 300].pack("C*").bytes, [1, -2, 300].pack("n*").bytes, [1].pack("Q<").bytes]
      assert_equal [[1, -2, 300], [1, 65534, 300], [1, -2, 300], [1, -2]], [[1, -2, 300].pack("l<*").unpack("l<*"), [1, -2, 300].pack("n*").unpack("n*"), [1, -2, 300].pack("q>*").unpack("q>*"), [1.7, -2].pack("c*").unpack("c*")]
      assert_equal [[1], [1, 0, 2], [72057594037927936]], ["\x01\x00\x00\x00\x00\x00\x00\x00\x02\x00\x00\x00".unpack("L_"), "\x01\x00\x00\x00\x00\x00\x00\x00\x02\x00\x00\x00".unpack("i*"), "\x01\x00\x00\x00\x00\x00\x00\x00".unpack("L!>")]
    end

    def test_pack_floats
      assert_equal [[0, 0, 192, 63], [63, 192, 0, 0], [0, 0, 0, 0, 0, 0, 248, 63], [63, 248, 0, 0, 0, 0, 0, 0], [0, 0, 192, 63, 0, 0, 0, 64]],
                   [[1.5].pack("e").bytes, [1.5].pack("g").bytes, [1.5].pack("E").bytes, [1.5].pack("G").bytes, [1.5, 2].pack("e*").bytes]
      s = [1.5, 2].pack("e*")
      assert_equal [[1.5, 2.0], [1.5, 2.0, nil], [2.000000474974513], [nil, nil]], [s.unpack("e*"), s.unpack("e3"), s.unpack("E"), "ab".unpack("e2")]
      assert_equal [[0.1], [-3.25], [1.5]], [[0.1].pack("D").unpack("D"), [-3.25].pack("G").unpack("G"), [1.5].pack("f").unpack("f")]
    end

    def test_pack_positions
      got = %w[x x3 Cx2C CXC C3X2 C@3C C3@1 x* X* @ @* C*@0].map { |f| [1, 2, 3].pack(f).bytes }
      assert_equal [[0], [0, 0, 0], [1, 0, 0, 2], [2], [1], [1, 0, 0, 2], [1], [], [], [0], [], []], got
      assert_equal [[], [nil], [], [97], [nil], [97, 97], [97, 97], [97, 98, 98]],
                   ["abc".unpack("x*"), "abc".unpack("x*C"), "abc".unpack("@"), "abc".unpack("@C"), "abc".unpack("@*C"), "abc".unpack("C@0C"), "abc".unpack("CXC"), "abc".unpack("C2X*C")]
    end

    def test_unpack_rest
      w = [0x81, 0, 0x7f, 0xa0, 0x80, 0x80, 0x80, 0x80, 0, 0x81].pack("C*")
      assert_equal [[128, 127, 1099511627776], [128], [128, 127]], [w.unpack("w*"), w.unpack("w"), w.unpack("w2")]
      assert_equal [["hello "], "h\xC3\xA9llo = w\xC3\xB6rld\tx \nnextzz=XYrest".bytes],
                   ["#:&5L\n#;&\\@\n".unpack("u"), "h=C3=A9llo =3D w=\n=C3=B6rld\tx =\n\nnext=\r\nzz=XYrest".unpack1("M").bytes]
      assert_equal [["hello"], ["hello", "hi"], ["hello"], ["hello"], ["hello"], ["a"], [""]],
                   ["aGVsbG8=\naGk=".unpack("m"), "aGVsbG8=\naGk=".unpack("mm"), "aGVsbG8=".unpack("m0"), "aGVsbG8".unpack("m"), "aGVsbG8=YQ==".unpack("m"), "YQ".unpack("m"), "Y".unpack("m")]
      assert_equal [[0xD800, 0x110000, 0x7fffffff], [""], 3], # U round-trips; u's length byte is cut to the input
                   [[0xD800, 0x110000, 0x7fffffff].pack("U*").unpack("U*"), "#".unpack("u"), "$AAAA".unpack1("u").bytes.size]
    end

    def test_pack_errors
      e = assert_raises(TypeError) { ["a"].pack("C") }
      assert_equal "no implicit conversion of String into Integer", e.message
      e = assert_raises(TypeError) { [1].pack("a") }
      assert_equal "no implicit conversion of Integer into String", e.message
      e = assert_raises(TypeError) { [nil].pack("C") }
      assert_equal "no implicit conversion of nil into Integer", e.message
      e = assert_raises(TypeError) { ["x"].pack("e") }
      assert_equal "can't convert String into Float", e.message
      e = assert_raises(ArgumentError) { [1].pack("C3") }
      assert_equal "too few arguments", e.message
      e = assert_raises(RangeError) { [-1].pack("U") }
      assert_equal "pack(U): value out of range", e.message
      e = assert_raises(ArgumentError) { [-1].pack("w") }
      assert_equal "can't compress negative numbers", e.message
      e = assert_raises(ArgumentError) { [1].pack("X") }
      assert_equal "X outside of string", e.message
      e = assert_raises(ArgumentError) { [1].pack("y") }
      assert_equal "unknown pack directive 'y' in 'y'", e.message
      e = assert_raises(ArgumentError) { [1].pack("C_") }
      assert_equal "'_' allowed only after types sSiIlLqQjJ", e.message
    end

    def test_unpack_errors
      e = assert_raises(ArgumentError) { [0xff].pack("C").unpack("U") }
      assert_equal "malformed UTF-8 character", e.message
      e = assert_raises(ArgumentError) { [0xC3].pack("C").unpack("U") }
      assert_equal "malformed UTF-8 character (expected 2 bytes, given 1 bytes)", e.message
      e = assert_raises(ArgumentError) { [0xC0, 0x80].pack("C*").unpack("U") }
      assert_equal "redundant UTF-8 sequence", e.message
      e = assert_raises(ArgumentError) { "aGVsbG8".unpack("m0") }
      assert_equal "invalid base64", e.message
      e = assert_raises(ArgumentError) { "aGk=\n".unpack("m0") }
      assert_equal "invalid base64", e.message
      e = assert_raises(ArgumentError) { "abc".unpack("X") }
      assert_equal "X outside of string", e.message
      e = assert_raises(ArgumentError) { "abc".unpack("x4") }
      assert_equal "x outside of string", e.message
      e = assert_raises(ArgumentError) { "abc".unpack("@4") }
      assert_equal "@ outside of string", e.message
      e = assert_raises(ArgumentError) { "ab".unpack("a<") }
      assert_equal "'<' allowed only after types sSiIlLqQjJ", e.message
      e = assert_raises(ArgumentError) { "ab".unpack("y") }
      assert_equal "unknown unpack directive 'y' in 'y'", e.message
    end
  end

  # ruby/spec core/string gaps (#49): undump
  class StringRubySpecUndumpTest < Minitest::Test
    def test_undump
      bs = 92.chr
      u = bs + "u"
      assert_equal "a\né😀AA\"\\\#{", ('"a' + bs + "n" + u + "00E9" + u + "{1F600 41}" + bs + "x41" + bs + '"' + bs + bs + bs + '#{"').undump
      s = "tab\t" + "é" + "\x00" + bs + "e" + 27.chr
      assert_equal s, s.dump.undump
      errors = ["abc", '"a', '"' + bs + 'xZZ"', '"é"'].map do |bad|
        bad.undump
      rescue RuntimeError => e
        e.message
      end
      assert_equal ["invalid dumped string; not wrapped with '\"' nor '\"...\".force_encoding(\"...\")' form", "unterminated dumped string", "invalid hex escape", "non-ASCII character detected"], errors
      assert_equal "a" + bs + "q", ('"a' + bs + 'q"').undump
    end
  end
end
