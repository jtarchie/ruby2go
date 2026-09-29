# frozen_string_literal: true
# rbs_inline: enabled

require "minitest/autorun"

# Helpers for the checks that were testdata/run/string_collections.rb.
#: (String) { (String) -> void } -> void
def each_vowel(s)
  s.each_char { |ch| yield ch if "aeiou".include?(ch) }
end

module StringFrozenTests
  # Helpers for the checks that were testdata/run/string_core.rb.
  # Decision 3: [] vs index, =~ vs match, -@/+@ and ! must not collide in Go.
  class Word
    attr_reader :s #: String

    #: (String) -> void
    def initialize(s)
      @s = s
    end

    #: (Integer) -> String?
    def [](i) = s[i]

    #: (String) -> Integer?
    def index(x) = s.index(x)

    #: (String) -> bool
    def =~(x) = s.include?(x)

    #: (String) -> bool
    def match(x) = s == x

    #: (Word) -> Integer
    def <=>(other) = s <=> other.s

    #: () -> Word
    def -@ = Word.new(s.reverse)

    #: () -> Word
    def +@ = Word.new(s.upcase)

    #: () -> bool
    def ! = s.empty?

    #: (Integer) -> String
    def idx_set(i) = "idx_set #{i}"
  end

  # Helpers for the checks that were testdata/run/string_constants.rb.
  # Strings and Symbols held in constants, ivars (attr_accessor, ||=) and default parameters.
  GREETING = "hi"

  MODE = :fast

  NAMES = %w[ann bob]

  SEP = ", " #: String

  class Person
    attr_accessor :name #: String
    attr_reader :nick #: String?

    #: (String) -> void
    def initialize(name)
      @name = name
      @nick = nil
    end

    #: () -> String
    def nick_or_name
      @nick ||= name.downcase
      @nick || ""
    end

    #: (?String, ?String) -> String
    def greet(greeting = GREETING, punct = "!") = "#{greeting}, #{name}#{punct}"
  end

  class StringCoreTest < Minitest::Test
    # <=> is a byte comparison returning -1/0/1; shorter prefix sorts first.
    def test_is_a_byte_comparison_returning
      assert_equal -1, ("a" <=> "b")
      assert_equal 1, ("b" <=> "a")
      assert_equal 0, ("a" <=> "a")
      assert_equal -1, ("a" <=> "ab")
      assert_equal -1, ("" <=> "a")
      assert_equal -1, ("B" <=> "a")
      assert_equal 1, ("é" <=> "z")
    end

    # == takes anything; other classes are never equal.
    def test_takes_anything_other_classes_are
      assert_equal true, ("a" == "a")
      assert_equal false, ("a" == "b")
      assert_equal false, ("a" == :a)
      assert_equal false, ("1" == 1)
      assert_equal false, ("" == nil)
      assert_equal true, ("a" != "b")
      assert_equal false, ("a" != "a")
      assert_equal false, (!"a")
      assert_equal false, (!"")
      u = "a" #: untyped
      assert_equal true, ("a" == u)
      assert_equal false, (:a == u)
      assert_equal false, ("b" == u)
    end

    # + and *
    def test_and
      assert_equal "abcd", ("ab" + "cd")
      assert_equal "", ("" + "")
      assert_equal "é😀", ("é" + "😀")
      assert_equal "ababab", ("ab" * 3)
      assert_equal "", ("ab" * 0)
      assert_equal "", ("" * 5)
      assert_equal "éé", ("é" * 2)
    end

    # to_s/to_str/dup return equal strings
    def test_to_s_to_str_dup
      assert_equal "abc", "abc".to_s
      assert_equal "abc", "abc".to_str
      assert_equal "abc", "abc".dup
    end

    # size counts characters, bytesize bytes
    def test_size_counts_characters_bytesize_bytes
      assert_equal 0, "".size
      assert_equal 5, "héllo".size
      assert_equal 5, "héllo".length
      assert_equal 1, "😀".size
      assert_equal 6, "héllo".bytesize
      assert_equal 4, "😀".bytesize
      assert_equal 0, "".bytesize
      assert_equal true, "".empty?
      assert_equal false, " ".empty?
      assert_equal false, "\n".empty?
    end

    # prefix/suffix/substring predicates, including the empty needle
    def test_prefix_suffix_substring_predicates_including
      assert_equal true, "hello".start_with?("he")
      assert_equal true, "hello".start_with?("")
      assert_equal false, "hello".start_with?("lo")
      assert_equal false, "".start_with?("a")
      assert_equal true, "hello".end_with?("lo")
      assert_equal true, "hello".end_with?("")
      assert_equal true, "héllo".end_with?("éllo")
      assert_equal false, "lo".end_with?("hello")
      assert_equal true, "hello".include?("ll")
      assert_equal true, "hello".include?("")
      assert_equal false, "hello".include?("L")
      assert_equal true, "日本語".include?("本")
    end

    # index is a character offset (not a byte offset) or nil
    def test_index_is_a_character_offset
      assert_equal 2, "héllo".index("l")
      assert_equal 0, "hello".index("")
      assert_nil "hello".index("z")
      assert_equal 2, "日本語".index("語")
      assert_equal 0, "".index("")
      assert_equal 1, "abab".index("b")
    end

    # [] with one Integer: negative counts from the end, out of range is nil
    def test_with_one_integer_negative_counts
      assert_equal "é", "héllo"[1]
      assert_equal "a", "abc"[0]
      assert_equal "c", "abc"[-1]
      assert_equal "a", "abc"[-3]
      assert_nil "abc"[-4]
      assert_nil "abc"[3]
      assert_nil ""[0]
      assert_equal "本", "日本"[-1]
      first = "xyz"[0]
      assert_equal "X", first.upcase if first
      assert_equal true, ("abc"[1] == "b")
      assert_equal true, "abc"[7].nil?
    end

    # Comparable on String
    def test_comparable_on_string
      assert_equal true, ("a" < "b")
      assert_equal true, ("a" <= "a")
      assert_equal true, ("b" > "a")
      assert_equal false, ("a" >= "b")
      assert_equal true, ("Z" < "a")
      assert_equal true, "m".between?("a", "z")
      assert_equal true, "m".between?("m", "m")
      assert_equal false, "A".between?("a", "z")
      assert_equal "f", "m".clamp("a", "f")
      assert_equal "c", "b".clamp("c", "f")
      assert_equal "d", "d".clamp("c", "f")
      assert_equal "c", "c".clamp("c", "c")
    end

    # hash is only comparable within a run
    def test_hash_is_only_comparable_within
      assert_equal true, ("abc".hash == "abc".hash)
      assert_equal false, ("abc".hash == "abd".hash)
      assert_equal true, ("a".hash == "a".dup.hash)
      assert_equal true, ("ab" + "c").hash == "abc".hash
    end

    # identity: literals are frozen and shared, dup and + make new strings
    def test_identity_literals_are_frozen_and
      t = "abc"
      assert_equal true, t.equal?(t)
      assert_equal false, t.equal?(t.dup)
      assert_equal true, (t.dup == t)
      assert_equal true, "abc".equal?("abc")
      assert_equal false, ("a" + "b").equal?("a" + "b")
      assert_equal true, ("a" + "b") == "ab"
      assert_equal true, "lit".frozen?
      assert_equal "x", "x".freeze
      assert_equal true, "x".freeze.equal?("x")
      assert_equal false, "hi".nil?
    end

    # Kernel#then on a String
    def test_kernel_then_on_a_string
      assert_equal "hi!", "hi".then { |x| x + "!" }
      assert_equal 4, "3".then { |x| x.to_i + 1 }
    end

    # sorting and min/max go through <=>
    def test_sorting_and_min_max_go
      assert_equal ["", "C", "a", "aa", "b"], ["b", "a", "C", "", "aa"].sort
      assert_equal "b", ["b", "a"].max
      assert_equal "a", ["b", "a"].min
    end

    # strings as hash keys compare by value
    def test_strings_as_hash_keys_compare
      counts = {} #: Hash[String, Integer]
      ["x", "y", "x", "x" + ""].each { |k| counts[k] = (counts[k] || 0) + 1 }
      assert_equal({"x" => 3, "y" => 1}, counts)
    end

    # case/when compares strings by value
    def test_case_when_compares_strings_by
      got = ["hi", "ho", "h" + "i", "?"].map do |w|
        case w
        when "ho" then "ho!"
        when "hi" then "hi!"
        else "other"
        end
      end
      assert_equal ["hi!", "ho!", "hi!", "other"], got
      w = Word.new("hello")
      assert_equal "e", w[1]
      assert_equal 2, w.index("l")
      assert_equal true, (w =~ "ell")
      assert_equal true, w.match("hello")
      assert_equal false, w.match("ell")
      assert_equal -1, (w <=> Word.new("world"))
      assert_equal "olleh", (-w).s
      assert_equal "HELLO", (+w).s
      assert_equal false, (!w)
      assert_equal true, (!Word.new(""))
      assert_equal "idx_set 2", w.idx_set(2)
    end
  end

  class StringCollectionsTest < Minitest::Test
    #: () -> Array[String]
    def words = %w[pear Apple fig banana kiwi]

    # Strings stored in collections and passed through blocks, &:sym and chained calls.
    def test_strings_stored_in_collections_and
      assert_equal ["PEAR", "APPLE", "FIG", "BANANA", "KIWI"], words.map(&:upcase)
      assert_equal ["fig"], words.select { |w| w.start_with?("f") }
      assert_equal ["fig", "pear", "kiwi", "Apple", "banana"], words.sort_by(&:size)
      assert_equal "banana", words.max_by(&:length)
      assert_equal "Apple", words.min_by { |w| w.downcase }
      assert_equal({4 => ["pear", "kiwi"], 5 => ["Apple"], 3 => ["fig"], 6 => ["banana"]}, words.group_by(&:size))
      assert_equal ["Apple", "banana", "fig", "kiwi", "pear"], words.sort
      assert_equal ["Apple", "banana", "fig", "kiwi", "pear"], words.sort_by(&:downcase)
      assert_equal "pear, Apple, fig, banana, kiwi", words.join(", ")
      assert_equal ["p", "A", "f", "b", "k"], words.map { |w| w[0] }
      assert_equal ["Raep", "Elppa", "Gif", "Ananab", "Iwik"], words.map(&:reverse).map(&:capitalize)
      assert_equal true, words.include?("kiwi")
      assert_equal false, words.include?("Kiwi")
      assert_equal true, words.include?("ki" + "wi")
      assert_equal({"pear" => 1, "Apple" => 1, "fig" => 1, "banana" => 1, "kiwi" => 1}, words.tally)
      assert_equal 5, words.uniq.size
      assert_equal 5, words.reject(&:empty?).size
      assert_equal true, words.any? { |w| w.end_with?("i") }
      assert_equal "|pear|Apple|fig|banana|kiwi", words.inject("") { |a, b| a + "|" + b }
      assert_equal "pear", words.max
      assert_equal "Apple", words.min
      assert_equal "fig", words.find { |w| w.size == 3 }
      assert_equal [:pear, :Apple, :fig, :banana, :kiwi], words.map(&:to_sym)
      assert_equal "\"pear\" \"Apple\" \"fig\" \"banana\" \"kiwi\"", words.map(&:inspect).join(" ")
      assert_equal 22, words.flat_map(&:chars).size
      out = ""
      words.each_with_index { |w, i| out += "#{i}#{w} " }
      assert_equal "0pear 1Apple 2fig 3banana 4kiwi ", out
    end

    # String? elements: || and && narrow each one.
    def test_string_elements_and_narrow_each
      opt = ["a", nil, "c"] #: Array[String?]
      assert_equal "a-c", opt.map { |x| x || "-" }.join
      assert_equal "A,nil,C", opt.map { |x| x && x.upcase }.map { |y| y || "nil" }.join(",")
    end

    # Rebinding with += / *= / ||= builds new strings (frozen literals are never mutated).
    def test_rebinding_with_builds_new_strings
      acc = ""
      words.each { |w| acc += w[0] || "" }
      assert_equal "pAfbk", acc
      acc2 = "x"
      acc2 += "y"
      acc2 *= 2
      assert_equal "xyxy", acc2
      name = nil #: String?
      name ||= "default"
      assert_equal "default", name
      name ||= "other"
      assert_equal "default", name
    end

    # &. on String?, and a narrowed element from a sub-array.
    def test_on_string_and_a_narrowed
      c = "abc"[9]
      assert_nil c&.upcase
      assert_equal -1, (c&.size || -1)
      assert_equal "A", "abc"[0]&.upcase
      first = words.first(1)[0]
      assert_equal "PEAR", first.upcase if first
    end

    # Hash[String, Array[String]] built by rebinding, and String keys by value.
    def test_hash_string_array_string_built
      cache = {} #: Hash[String, Array[String]]
      words.each do |w|
        k = w[0] || ""
        cache[k] = (cache[k] || []) + [w]
      end
      assert_equal({"p" => ["pear"], "A" => ["Apple"], "f" => ["fig"], "b" => ["banana"], "k" => ["kiwi"]}, cache)
      assert_equal ["pear"], cache["p"]
      assert_equal ["pear"], cache["p" + ""]
      assert_equal false, cache.key?("z")
      lens = {} #: Hash[String, Integer]
      words.each { |w| lens[w.downcase] = w.size }
      assert_equal({"pear" => 4, "apple" => 5, "fig" => 3, "banana" => 6, "kiwi" => 4}, lens)
      assert_equal ["PEAR", "APPLE", "FIG", "BANANA", "KIWI"], lens.keys.map(&:upcase)
      assert_equal "pear=4&apple=5&fig=3&banana=6&kiwi=4", lens.map { |k, v| "#{k}=#{v}" }.join("&")
      vowels = [] #: Array[String]
      each_vowel("education") { |v| vowels << v.upcase }
      assert_equal %w[E U A I O], vowels
      assert_equal "  xxx  ", "x".then { |s| s * 3 }.then { |s| s.center(7) }
      nested = [["b", "a"], ["d", "c"]] #: Array[Array[String]]
      assert_equal ["ab", "cd"], nested.map { |pair| pair.sort.join }
      assert_equal ["b", "d"], nested.map { |pair| pair[0] }
    end

    # split into multiple assignment (a missing part is nil), line-oriented chains, tuple sort keys.
    def test_split_into_multiple_assignment_a
      k, v = "key=val".split("=")
      assert_equal "key", k
      assert_equal "val", v
      solo, rest = "solo".split(",")
      assert_equal "solo", solo
      assert_nil rest
      text = "  one \n two\n\n three  \n"
      assert_equal ["  one ", " two", "", " three  "], text.lines.map(&:chomp)
      assert_equal ["one", "two", "three"], text.split("\n").map(&:strip).reject(&:empty?)
      assert_equal ["banana", "Apple", "kiwi", "pear", "fig"], words.sort_by { |w| [-w.size, w] }
      assert_equal ["g", "b", "fig", "banana", "g"], words.map { |w| w.clamp("b", "g") }
      assert_equal ["fig", "banana"], words.select { |w| w.between?("b", "g") }
      fields = "name: Ann, age: 30".split(", ").map { |f| f.split(": ") }
      assert_equal [["name", "Ann"], ["age", "30"]], fields
      assert_equal ["Ann", "30"], fields.map { |f| f[1] }
      parts = [] #: Array[String]
      "a,b;c".split(";").each { |part| part.split(",").each { |x| parts << x.upcase } }
      assert_equal ["A", "B", "C"], parts
      out = ""
      "ab".chars.each_with_index { |ch, i| out += ch * (i + 1) + " " }
      assert_equal "a bb ", out
    end

    # each_char as a statement inside a value block (a closure).
    def test_each_char_as_a_statement
      per = %w[abc bb].map do |w|
        hits = 0
        w.each_char { |ch| hits += 1 unless ch == "b" }
        hits
      end
      assert_equal [2, 0], per
    end

    # == on String? both ways, &. chains, and narrowing an index before using it.
    def test_on_string_both_ways_chains
      none = "abc"[9]
      one = "abc"[0]
      assert_equal false, (none == "a")
      assert_equal true, (one == "a")
      assert_equal true, (none == nil)
      assert_nil "abc"[9]&.upcase&.downcase
      assert_equal "b", "abc"[1]&.upcase&.downcase
      hello = "hello"
      at = hello.index("l")
      assert_equal "l", hello[at] if at
      assert_equal -1, (hello.index("z") || -1)
      assert_equal 0, (hello.index("h") || -1)
    end

    # String == against String? values from [] and Hash#[], and on strings built in a block.
    def test_string_against_string_values_from
      assert_equal true, ("a" == "abc"[0])
      assert_equal false, ("a" == "abc"[9])
      assert_equal true, (:a == [:a][0])
      kv = { "k" => "v" } #: Hash[String, String]
      assert_equal true, ("v" == kv["k"])
      assert_equal true, (kv["k"] == "v")
      assert_equal false, (kv["zz"] == "v")
      built = ["a", "b"].map { |ch| ch + "" }
      assert_equal true, built.include?("a")
      assert_equal true, (built[0] == "a")
      assert_equal true, (built == ["a", "b"])
    end
  end

  class StringConstantsTest < Minitest::Test
    def test_strings_and_symbols_in_constants
      assert_equal "HI", GREETING.upcase
      assert_equal true, GREETING.frozen?
      assert_equal "hi fast", "#{GREETING} #{MODE}"
      assert_equal :fast, MODE
      assert_equal "ann, bob", NAMES.join(SEP)
      assert_equal ["Ann", "Bob"], NAMES.map(&:capitalize)
      said = "h" + "i"
      matched = case said
                when GREETING then "matched const"
                else "no"
                end
      assert_equal "matched const", matched
      m = :fast
      mode = case m
             when MODE then "mode const"
             end
      assert_equal "mode const", mode
      p1 = Person.new("Ann")
      assert_equal "hi, Ann!", p1.greet
      assert_equal "yo, Ann!", p1.greet("yo")
      assert_equal "hey, Ann?", p1.greet("hey", "?")
      assert_equal "ann", p1.nick_or_name
      assert_equal "ann", p1.nick
      p1.name = p1.name + " Lee"
      assert_equal "Ann Lee", p1.name
      assert_equal "hi, Ann Lee!", p1.greet
      assert_equal true, GREETING.equal?(GREETING)
    end
  end

  class StringBugEqualSharedDataTest < Minitest::Test
    # A String method's result is a new object even when its bytes equal the receiver's.
    def test_results_are_new_objects
      x = "ab"
      assert_equal false, x.strip.equal?(x)
      assert_equal false, x.sub("z", "y").equal?(x)
      assert_equal false, x.gsub("z", "y").equal?(x)
      assert_equal false, x.center(1).equal?(x)
      assert_equal false, x.chomp.equal?(x)
      assert_equal false, x.downcase.equal?(x)
      assert_equal false, x.tr("z", "y").equal?(x)
      assert_equal false, x.split(",")[0].equal?(x)
      assert_equal false, x.lines[0].equal?(x)
      assert_equal false, (x * 1).equal?(x)
      assert_equal false, "#{x}".equal?(x)
      assert_equal false, :a.to_s.equal?(:a.to_s)
      assert_equal false, x.to_sym.to_s.equal?(x)
      e = ""
      assert_equal false, e.equal?(e.dup)
    end
  end

  class StringBugFrozenComputedTest < Minitest::Test
    # Only literals are frozen under the pragma; computed strings aren't.
    def test_only_literals_are_frozen
      assert_equal true, "lit".frozen?
      assert_equal false, ("a" + "b").frozen?
      assert_equal false, "x".dup.frozen?
    end
  end
end
