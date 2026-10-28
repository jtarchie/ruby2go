# rbs_inline: enabled

require "minitest/autorun"

module RegexpTests
  # gsub/sub/scan/split apply Onigmo's ^ and \b readings (decision 24), not only match/=~.
  CVE_PATTERN = /CVE-?(\d{4})-(\d{4,})/i
  PAIR_PATTERN = /(\w)=(\w)/

  class RegexpFindAllTest < Minitest::Test
    # scan through a constant holding a literal with groups yields the groups, as the literal would (Homebrew's
    # Patch.extract_cves)
    def test_scan_groups_through_constant
      got = "fix CVE-2024-1234 and cve-2023-99999".scan(CVE_PATTERN).map { |y, id| "#{y}/#{id}" }
      assert_equal ["2024/1234", "2023/99999"], got
      assert_equal [["a", "b"], ["c", "d"]], "a=b c=d".scan(PAIR_PATTERN)
    end

    # \b is backspace inside a class, a word boundary outside; Go's parser has no \b escape in a class.
    def test_backspace_in_class
      assert_equal "aXb", "a\bb".sub(/[\b]/, "X")
      assert_nil "ab".match(/[\b]/)
      assert_equal "a\b", "a\bb"[/a[x\by]/]
      assert_equal 0, "a b" =~ /a\b/
    end

    def test_c1
      s = "a\n"
      assert_equal "> a\n", s.gsub(/^/, "> ") # ^
      assert_equal "> a\n", s.sub(/^/, "> ")
      assert_equal "<>a\n", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/^/)
      assert_equal ["a\n"], s.split(/^/)
      assert_equal ["a\n"], s.split(/^/, -1)
    end

    def test_c2
      s = "a\n"
      assert_equal "> a\n", s.gsub(/^/m, "> ") # ^m
      assert_equal "> a\n", s.sub(/^/m, "> ")
      assert_equal "<>a\n", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/^/m)
      assert_equal ["a\n"], s.split(/^/m)
      assert_equal ["a\n"], s.split(/^/m, -1)
    end

    def test_c3
      s = "a\n"
      assert_equal "> a> \n", s.gsub(/\b/, "> ") # b
      assert_equal "> a\n", s.sub(/\b/, "> ")
      assert_equal "<>a<>\n", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/\b/)
      assert_equal ["a", "\n"], s.split(/\b/)
      assert_equal ["a", "\n"], s.split(/\b/, -1)
    end

    def test_c4
      s = "a\n"
      assert_equal "> a> \n", s.gsub(/\b/m, "> ") # bm
      assert_equal "> a\n", s.sub(/\b/m, "> ")
      assert_equal "<>a<>\n", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/\b/m)
      assert_equal ["a", "\n"], s.split(/\b/m)
      assert_equal ["a", "\n"], s.split(/\b/m, -1)
    end

    def test_c5
      s = "a\n"
      assert_equal "> \n", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal "> \n", s.sub(/\bé|\ba/, "> ")
      assert_equal "<a>\n", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/\bé|\ba/)
      assert_equal ["", "\n"], s.split(/\bé|\ba/)
      assert_equal ["", "\n"], s.split(/\bé|\ba/, -1)
    end

    def test_c6
      s = "a\n"
      assert_equal "> \n", s.gsub(/^a/, "> ") # caret_a
      assert_equal "> \n", s.sub(/^a/, "> ")
      assert_equal "<a>\n", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/^a/)
      assert_equal ["", "\n"], s.split(/^a/)
      assert_equal ["", "\n"], s.split(/^a/, -1)
    end

    def test_c7
      s = "a\n"
      assert_equal "> a> \n> ", s.gsub(/b*/, "> ") # x*
      assert_equal "> a\n", s.sub(/b*/, "> ")
      assert_equal "<>a<>\n<>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal ["", "", ""], s.scan(/b*/)
      assert_equal ["a", "\n"], s.split(/b*/)
      assert_equal ["a", "\n", ""], s.split(/b*/, -1)
    end

    def test_c8
      s = "a\n"
      assert_equal "> a\n", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> a\n", s.sub(/(^)/, "> ")
      assert_equal "<>a\n", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""]], s.scan(/(^)/)
      assert_equal ["a\n"], s.split(/(^)/)
      assert_equal ["a\n"], s.split(/(^)/, -1)
    end

    def test_c9
      s = "a\n"
      assert_equal "> a> \n> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> a\n", s.sub(/^|$/, "> ")
      assert_equal "<>a<>\n<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal ["", "", ""], s.scan(/^|$/)
      assert_equal ["a", "\n"], s.split(/^|$/)
      assert_equal ["a", "\n", ""], s.split(/^|$/, -1)
    end

    def test_c10
      s = "a\nb\n"
      assert_equal "> a\n> b\n", s.gsub(/^/, "> ") # ^
      assert_equal "> a\nb\n", s.sub(/^/, "> ")
      assert_equal "<>a\n<>b\n", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/)
      assert_equal ["a\n", "b\n"], s.split(/^/)
      assert_equal ["a\n", "b\n"], s.split(/^/, -1)
    end

    def test_c11
      s = "a\nb\n"
      assert_equal "> a\n> b\n", s.gsub(/^/m, "> ") # ^m
      assert_equal "> a\nb\n", s.sub(/^/m, "> ")
      assert_equal "<>a\n<>b\n", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/m)
      assert_equal ["a\n", "b\n"], s.split(/^/m)
      assert_equal ["a\n", "b\n"], s.split(/^/m, -1)
    end

    def test_c12
      s = "a\nb\n"
      assert_equal "> a> \n> b> \n", s.gsub(/\b/, "> ") # b
      assert_equal "> a\nb\n", s.sub(/\b/, "> ")
      assert_equal "<>a<>\n<>b<>\n", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/\b/)
      assert_equal ["a", "\n", "b", "\n"], s.split(/\b/)
      assert_equal ["a", "\n", "b", "\n"], s.split(/\b/, -1)
    end

    def test_c13
      s = "a\nb\n"
      assert_equal "> a> \n> b> \n", s.gsub(/\b/m, "> ") # bm
      assert_equal "> a\nb\n", s.sub(/\b/m, "> ")
      assert_equal "<>a<>\n<>b<>\n", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/\b/m)
      assert_equal ["a", "\n", "b", "\n"], s.split(/\b/m)
      assert_equal ["a", "\n", "b", "\n"], s.split(/\b/m, -1)
    end

    def test_c14
      s = "a\nb\n"
      assert_equal "> \nb\n", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal "> \nb\n", s.sub(/\bé|\ba/, "> ")
      assert_equal "<a>\nb\n", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/\bé|\ba/)
      assert_equal ["", "\nb\n"], s.split(/\bé|\ba/)
      assert_equal ["", "\nb\n"], s.split(/\bé|\ba/, -1)
    end

    def test_c15
      s = "a\nb\n"
      assert_equal "> \nb\n", s.gsub(/^a/, "> ") # caret_a
      assert_equal "> \nb\n", s.sub(/^a/, "> ")
      assert_equal "<a>\nb\n", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/^a/)
      assert_equal ["", "\nb\n"], s.split(/^a/)
      assert_equal ["", "\nb\n"], s.split(/^a/, -1)
    end

    def test_c16
      s = "a\nb\n"
      assert_equal "> a> \n> > \n> ", s.gsub(/b*/, "> ") # x*
      assert_equal "> a\nb\n", s.sub(/b*/, "> ")
      assert_equal "<>a<>\n<b><>\n<>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal ["", "", "b", "", ""], s.scan(/b*/)
      assert_equal ["a", "\n", "\n"], s.split(/b*/)
      assert_equal ["a", "\n", "\n", ""], s.split(/b*/, -1)
    end

    def test_c17
      s = "a\nb\n"
      assert_equal "> a\n> b\n", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> a\nb\n", s.sub(/(^)/, "> ")
      assert_equal "<>a\n<>b\n", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""], [""]], s.scan(/(^)/)
      assert_equal ["a\n", "", "b\n"], s.split(/(^)/)
      assert_equal ["a\n", "", "b\n"], s.split(/(^)/, -1)
    end

    def test_c18
      s = "a\nb\n"
      assert_equal "> a> \n> b> \n> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> a\nb\n", s.sub(/^|$/, "> ")
      assert_equal "<>a<>\n<>b<>\n<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal ["", "", "", "", ""], s.scan(/^|$/)
      assert_equal ["a", "\n", "b", "\n"], s.split(/^|$/)
      assert_equal ["a", "\n", "b", "\n", ""], s.split(/^|$/, -1)
    end

    def test_c19
      s = ""
      assert_equal "> ", s.gsub(/^/, "> ") # ^
      assert_equal "> ", s.sub(/^/, "> ")
      assert_equal "<>", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/^/)
      assert_equal [], s.split(/^/)
      assert_equal [], s.split(/^/, -1)
    end

    def test_c20
      s = ""
      assert_equal "> ", s.gsub(/^/m, "> ") # ^m
      assert_equal "> ", s.sub(/^/m, "> ")
      assert_equal "<>", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/^/m)
      assert_equal [], s.split(/^/m)
      assert_equal [], s.split(/^/m, -1)
    end

    def test_c21
      s = ""
      assert_equal "", s.gsub(/\b/, "> ") # b
      assert_equal "", s.sub(/\b/, "> ")
      assert_equal "", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/\b/)
      assert_equal [], s.split(/\b/)
      assert_equal [], s.split(/\b/, -1)
    end

    def test_c22
      s = ""
      assert_equal "", s.gsub(/\b/m, "> ") # bm
      assert_equal "", s.sub(/\b/m, "> ")
      assert_equal "", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal [], s.scan(/\b/m)
      assert_equal [], s.split(/\b/m)
      assert_equal [], s.split(/\b/m, -1)
    end

    def test_c23
      s = ""
      assert_equal "", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal "", s.sub(/\bé|\ba/, "> ")
      assert_equal "", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/\bé|\ba/)
      assert_equal [], s.split(/\bé|\ba/)
      assert_equal [], s.split(/\bé|\ba/, -1)
    end

    def test_c24
      s = ""
      assert_equal "", s.gsub(/^a/, "> ") # caret_a
      assert_equal "", s.sub(/^a/, "> ")
      assert_equal "", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/^a/)
      assert_equal [], s.split(/^a/)
      assert_equal [], s.split(/^a/, -1)
    end

    def test_c25
      s = ""
      assert_equal "> ", s.gsub(/b*/, "> ") # x*
      assert_equal "> ", s.sub(/b*/, "> ")
      assert_equal "<>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/b*/)
      assert_equal [], s.split(/b*/)
      assert_equal [], s.split(/b*/, -1)
    end

    def test_c26
      s = ""
      assert_equal "> ", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> ", s.sub(/(^)/, "> ")
      assert_equal "<>", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""]], s.scan(/(^)/)
      assert_equal [], s.split(/(^)/)
      assert_equal [], s.split(/(^)/, -1)
    end

    def test_c27
      s = ""
      assert_equal "> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> ", s.sub(/^|$/, "> ")
      assert_equal "<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/^|$/)
      assert_equal [], s.split(/^|$/)
      assert_equal [], s.split(/^|$/, -1)
    end

    def test_c28
      s = "\n"
      assert_equal "> \n", s.gsub(/^/, "> ") # ^
      assert_equal "> \n", s.sub(/^/, "> ")
      assert_equal "<>\n", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/^/)
      assert_equal ["\n"], s.split(/^/)
      assert_equal ["\n"], s.split(/^/, -1)
    end

    def test_c29
      s = "\n"
      assert_equal "> \n", s.gsub(/^/m, "> ") # ^m
      assert_equal "> \n", s.sub(/^/m, "> ")
      assert_equal "<>\n", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal [""], s.scan(/^/m)
      assert_equal ["\n"], s.split(/^/m)
      assert_equal ["\n"], s.split(/^/m, -1)
    end

    def test_c30
      s = "\n"
      assert_equal "\n", s.gsub(/\b/, "> ") # b
      assert_equal "\n", s.sub(/\b/, "> ")
      assert_equal "\n", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/\b/)
      assert_equal ["\n"], s.split(/\b/)
      assert_equal ["\n"], s.split(/\b/, -1)
    end

    def test_c31
      s = "\n"
      assert_equal "\n", s.gsub(/\b/m, "> ") # bm
      assert_equal "\n", s.sub(/\b/m, "> ")
      assert_equal "\n", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal [], s.scan(/\b/m)
      assert_equal ["\n"], s.split(/\b/m)
      assert_equal ["\n"], s.split(/\b/m, -1)
    end

    def test_c32
      s = "\n"
      assert_equal "\n", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal "\n", s.sub(/\bé|\ba/, "> ")
      assert_equal "\n", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/\bé|\ba/)
      assert_equal ["\n"], s.split(/\bé|\ba/)
      assert_equal ["\n"], s.split(/\bé|\ba/, -1)
    end

    def test_c33
      s = "\n"
      assert_equal "\n", s.gsub(/^a/, "> ") # caret_a
      assert_equal "\n", s.sub(/^a/, "> ")
      assert_equal "\n", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/^a/)
      assert_equal ["\n"], s.split(/^a/)
      assert_equal ["\n"], s.split(/^a/, -1)
    end

    def test_c34
      s = "\n"
      assert_equal "> \n> ", s.gsub(/b*/, "> ") # x*
      assert_equal "> \n", s.sub(/b*/, "> ")
      assert_equal "<>\n<>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/b*/)
      assert_equal ["\n"], s.split(/b*/)
      assert_equal ["\n", ""], s.split(/b*/, -1)
    end

    def test_c35
      s = "\n"
      assert_equal "> \n", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> \n", s.sub(/(^)/, "> ")
      assert_equal "<>\n", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""]], s.scan(/(^)/)
      assert_equal ["\n"], s.split(/(^)/)
      assert_equal ["\n"], s.split(/(^)/, -1)
    end

    def test_c36
      s = "\n"
      assert_equal "> \n> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> \n", s.sub(/^|$/, "> ")
      assert_equal "<>\n<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^|$/)
      assert_equal ["\n"], s.split(/^|$/)
      assert_equal ["\n", ""], s.split(/^|$/, -1)
    end

    def test_c37
      s = "é a\nb é\n"
      assert_equal "> é a\n> b é\n", s.gsub(/^/, "> ") # ^
      assert_equal "> é a\nb é\n", s.sub(/^/, "> ")
      assert_equal "<>é a\n<>b é\n", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/)
      assert_equal ["é a\n", "b é\n"], s.split(/^/)
      assert_equal ["é a\n", "b é\n"], s.split(/^/, -1)
    end

    def test_c38
      s = "é a\nb é\n"
      assert_equal "> é a\n> b é\n", s.gsub(/^/m, "> ") # ^m
      assert_equal "> é a\nb é\n", s.sub(/^/m, "> ")
      assert_equal "<>é a\n<>b é\n", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/m)
      assert_equal ["é a\n", "b é\n"], s.split(/^/m)
      assert_equal ["é a\n", "b é\n"], s.split(/^/m, -1)
    end

    def test_c39
      s = "é a\nb é\n"
      assert_equal "> é>  > a> \n> b>  > é> \n", s.gsub(/\b/, "> ") # b
      assert_equal "> é a\nb é\n", s.sub(/\b/, "> ")
      assert_equal "<>é<> <>a<>\n<>b<> <>é<>\n", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal ["", "", "", "", "", "", "", ""], s.scan(/\b/)
      assert_equal ["é", " ", "a", "\n", "b", " ", "é", "\n"], s.split(/\b/)
      assert_equal ["é", " ", "a", "\n", "b", " ", "é", "\n"], s.split(/\b/, -1)
    end

    def test_c40
      s = "é a\nb é\n"
      assert_equal "> é>  > a> \n> b>  > é> \n", s.gsub(/\b/m, "> ") # bm
      assert_equal "> é a\nb é\n", s.sub(/\b/m, "> ")
      assert_equal "<>é<> <>a<>\n<>b<> <>é<>\n", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal ["", "", "", "", "", "", "", ""], s.scan(/\b/m)
      assert_equal ["é", " ", "a", "\n", "b", " ", "é", "\n"], s.split(/\b/m)
      assert_equal ["é", " ", "a", "\n", "b", " ", "é", "\n"], s.split(/\b/m, -1)
    end

    def test_c41
      s = "é a\nb é\n"
      assert_equal ">  > \nb > \n", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal ">  a\nb é\n", s.sub(/\bé|\ba/, "> ")
      assert_equal "<é> <a>\nb <é>\n", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal ["é", "a", "é"], s.scan(/\bé|\ba/)
      assert_equal ["", " ", "\nb ", "\n"], s.split(/\bé|\ba/)
      assert_equal ["", " ", "\nb ", "\n"], s.split(/\bé|\ba/, -1)
    end

    def test_c42
      s = "é a\nb é\n"
      assert_equal "é a\nb é\n", s.gsub(/^a/, "> ") # caret_a
      assert_equal "é a\nb é\n", s.sub(/^a/, "> ")
      assert_equal "é a\nb é\n", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/^a/)
      assert_equal ["é a\nb é\n"], s.split(/^a/)
      assert_equal ["é a\nb é\n"], s.split(/^a/, -1)
    end

    def test_c43
      s = "é a\nb é\n"
      assert_equal "> é>  > a> \n> >  > é> \n> ", s.gsub(/b*/, "> ") # x*
      assert_equal "> é a\nb é\n", s.sub(/b*/, "> ")
      assert_equal "<>é<> <>a<>\n<b><> <>é<>\n<>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal ["", "", "", "", "b", "", "", "", ""], s.scan(/b*/)
      assert_equal ["é", " ", "a", "\n", " ", "é", "\n"], s.split(/b*/)
      assert_equal ["é", " ", "a", "\n", " ", "é", "\n", ""], s.split(/b*/, -1)
    end

    def test_c44
      s = "é a\nb é\n"
      assert_equal "> é a\n> b é\n", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> é a\nb é\n", s.sub(/(^)/, "> ")
      assert_equal "<>é a\n<>b é\n", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""], [""]], s.scan(/(^)/)
      assert_equal ["é a\n", "", "b é\n"], s.split(/(^)/)
      assert_equal ["é a\n", "", "b é\n"], s.split(/(^)/, -1)
    end

    def test_c45
      s = "é a\nb é\n"
      assert_equal "> é a> \n> b é> \n> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> é a\nb é\n", s.sub(/^|$/, "> ")
      assert_equal "<>é a<>\n<>b é<>\n<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal ["", "", "", "", ""], s.scan(/^|$/)
      assert_equal ["é a", "\n", "b é", "\n"], s.split(/^|$/)
      assert_equal ["é a", "\n", "b é", "\n", ""], s.split(/^|$/, -1)
    end

    def test_c46
      s = "ab\n\n"
      assert_equal "> ab\n> \n", s.gsub(/^/, "> ") # ^
      assert_equal "> ab\n\n", s.sub(/^/, "> ")
      assert_equal "<>ab\n<>\n", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/)
      assert_equal ["ab\n", "\n"], s.split(/^/)
      assert_equal ["ab\n", "\n"], s.split(/^/, -1)
    end

    def test_c47
      s = "ab\n\n"
      assert_equal "> ab\n> \n", s.gsub(/^/m, "> ") # ^m
      assert_equal "> ab\n\n", s.sub(/^/m, "> ")
      assert_equal "<>ab\n<>\n", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/m)
      assert_equal ["ab\n", "\n"], s.split(/^/m)
      assert_equal ["ab\n", "\n"], s.split(/^/m, -1)
    end

    def test_c48
      s = "ab\n\n"
      assert_equal "> ab> \n\n", s.gsub(/\b/, "> ") # b
      assert_equal "> ab\n\n", s.sub(/\b/, "> ")
      assert_equal "<>ab<>\n\n", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/\b/)
      assert_equal ["ab", "\n\n"], s.split(/\b/)
      assert_equal ["ab", "\n\n"], s.split(/\b/, -1)
    end

    def test_c49
      s = "ab\n\n"
      assert_equal "> ab> \n\n", s.gsub(/\b/m, "> ") # bm
      assert_equal "> ab\n\n", s.sub(/\b/m, "> ")
      assert_equal "<>ab<>\n\n", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/\b/m)
      assert_equal ["ab", "\n\n"], s.split(/\b/m)
      assert_equal ["ab", "\n\n"], s.split(/\b/m, -1)
    end

    def test_c50
      s = "ab\n\n"
      assert_equal "> b\n\n", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal "> b\n\n", s.sub(/\bé|\ba/, "> ")
      assert_equal "<a>b\n\n", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/\bé|\ba/)
      assert_equal ["", "b\n\n"], s.split(/\bé|\ba/)
      assert_equal ["", "b\n\n"], s.split(/\bé|\ba/, -1)
    end

    def test_c51
      s = "ab\n\n"
      assert_equal "> b\n\n", s.gsub(/^a/, "> ") # caret_a
      assert_equal "> b\n\n", s.sub(/^a/, "> ")
      assert_equal "<a>b\n\n", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/^a/)
      assert_equal ["", "b\n\n"], s.split(/^a/)
      assert_equal ["", "b\n\n"], s.split(/^a/, -1)
    end

    def test_c52
      s = "ab\n\n"
      assert_equal "> a> > \n> \n> ", s.gsub(/b*/, "> ") # x*
      assert_equal "> ab\n\n", s.sub(/b*/, "> ")
      assert_equal "<>a<b><>\n<>\n<>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal ["", "b", "", "", ""], s.scan(/b*/)
      assert_equal ["a", "\n", "\n"], s.split(/b*/)
      assert_equal ["a", "\n", "\n", ""], s.split(/b*/, -1)
    end

    def test_c53
      s = "ab\n\n"
      assert_equal "> ab\n> \n", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> ab\n\n", s.sub(/(^)/, "> ")
      assert_equal "<>ab\n<>\n", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""], [""]], s.scan(/(^)/)
      assert_equal ["ab\n", "", "\n"], s.split(/(^)/)
      assert_equal ["ab\n", "", "\n"], s.split(/(^)/, -1)
    end

    def test_c54
      s = "ab\n\n"
      assert_equal "> ab> \n> \n> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> ab\n\n", s.sub(/^|$/, "> ")
      assert_equal "<>ab<>\n<>\n<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/^|$/)
      assert_equal ["ab", "\n", "\n"], s.split(/^|$/)
      assert_equal ["ab", "\n", "\n", ""], s.split(/^|$/, -1)
    end

    def test_c55
      s = "a\n\nb"
      assert_equal "> a\n> \n> b", s.gsub(/^/, "> ") # ^
      assert_equal "> a\n\nb", s.sub(/^/, "> ")
      assert_equal "<>a\n<>\n<>b", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal ["", "", ""], s.scan(/^/)
      assert_equal ["a\n", "\n", "b"], s.split(/^/)
      assert_equal ["a\n", "\n", "b"], s.split(/^/, -1)
    end

    def test_c56
      s = "a\n\nb"
      assert_equal "> a\n> \n> b", s.gsub(/^/m, "> ") # ^m
      assert_equal "> a\n\nb", s.sub(/^/m, "> ")
      assert_equal "<>a\n<>\n<>b", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal ["", "", ""], s.scan(/^/m)
      assert_equal ["a\n", "\n", "b"], s.split(/^/m)
      assert_equal ["a\n", "\n", "b"], s.split(/^/m, -1)
    end

    def test_c57
      s = "a\n\nb"
      assert_equal "> a> \n\n> b> ", s.gsub(/\b/, "> ") # b
      assert_equal "> a\n\nb", s.sub(/\b/, "> ")
      assert_equal "<>a<>\n\n<>b<>", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/\b/)
      assert_equal ["a", "\n\n", "b"], s.split(/\b/)
      assert_equal ["a", "\n\n", "b", ""], s.split(/\b/, -1)
    end

    def test_c58
      s = "a\n\nb"
      assert_equal "> a> \n\n> b> ", s.gsub(/\b/m, "> ") # bm
      assert_equal "> a\n\nb", s.sub(/\b/m, "> ")
      assert_equal "<>a<>\n\n<>b<>", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/\b/m)
      assert_equal ["a", "\n\n", "b"], s.split(/\b/m)
      assert_equal ["a", "\n\n", "b", ""], s.split(/\b/m, -1)
    end

    def test_c59
      s = "a\n\nb"
      assert_equal "> \n\nb", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal "> \n\nb", s.sub(/\bé|\ba/, "> ")
      assert_equal "<a>\n\nb", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/\bé|\ba/)
      assert_equal ["", "\n\nb"], s.split(/\bé|\ba/)
      assert_equal ["", "\n\nb"], s.split(/\bé|\ba/, -1)
    end

    def test_c60
      s = "a\n\nb"
      assert_equal "> \n\nb", s.gsub(/^a/, "> ") # caret_a
      assert_equal "> \n\nb", s.sub(/^a/, "> ")
      assert_equal "<a>\n\nb", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal ["a"], s.scan(/^a/)
      assert_equal ["", "\n\nb"], s.split(/^a/)
      assert_equal ["", "\n\nb"], s.split(/^a/, -1)
    end

    def test_c61
      s = "a\n\nb"
      assert_equal "> a> \n> \n> > ", s.gsub(/b*/, "> ") # x*
      assert_equal "> a\n\nb", s.sub(/b*/, "> ")
      assert_equal "<>a<>\n<>\n<b><>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal ["", "", "", "b", ""], s.scan(/b*/)
      assert_equal ["a", "\n", "\n"], s.split(/b*/)
      assert_equal ["a", "\n", "\n", ""], s.split(/b*/, -1)
    end

    def test_c62
      s = "a\n\nb"
      assert_equal "> a\n> \n> b", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> a\n\nb", s.sub(/(^)/, "> ")
      assert_equal "<>a\n<>\n<>b", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""], [""], [""]], s.scan(/(^)/)
      assert_equal ["a\n", "", "\n", "", "b"], s.split(/(^)/)
      assert_equal ["a\n", "", "\n", "", "b"], s.split(/(^)/, -1)
    end

    def test_c63
      s = "a\n\nb"
      assert_equal "> a> \n> \n> b> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> a\n\nb", s.sub(/^|$/, "> ")
      assert_equal "<>a<>\n<>\n<>b<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal ["", "", "", "", ""], s.scan(/^|$/)
      assert_equal ["a", "\n", "\n", "b"], s.split(/^|$/)
      assert_equal ["a", "\n", "\n", "b", ""], s.split(/^|$/, -1)
    end

    def test_c64
      s = "été\nnoël"
      assert_equal "> été\n> noël", s.gsub(/^/, "> ") # ^
      assert_equal "> été\nnoël", s.sub(/^/, "> ")
      assert_equal "<>été\n<>noël", s.gsub(/^/) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/)
      assert_equal ["été\n", "noël"], s.split(/^/)
      assert_equal ["été\n", "noël"], s.split(/^/, -1)
    end

    def test_c65
      s = "été\nnoël"
      assert_equal "> été\n> noël", s.gsub(/^/m, "> ") # ^m
      assert_equal "> été\nnoël", s.sub(/^/m, "> ")
      assert_equal "<>été\n<>noël", s.gsub(/^/m) { |m| "<#{m}>" }
      assert_equal ["", ""], s.scan(/^/m)
      assert_equal ["été\n", "noël"], s.split(/^/m)
      assert_equal ["été\n", "noël"], s.split(/^/m, -1)
    end

    def test_c66
      s = "été\nnoël"
      assert_equal "> été> \n> noël> ", s.gsub(/\b/, "> ") # b
      assert_equal "> été\nnoël", s.sub(/\b/, "> ")
      assert_equal "<>été<>\n<>noël<>", s.gsub(/\b/) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/\b/)
      assert_equal ["été", "\n", "noël"], s.split(/\b/)
      assert_equal ["été", "\n", "noël", ""], s.split(/\b/, -1)
    end

    def test_c67
      s = "été\nnoël"
      assert_equal "> été> \n> noël> ", s.gsub(/\b/m, "> ") # bm
      assert_equal "> été\nnoël", s.sub(/\b/m, "> ")
      assert_equal "<>été<>\n<>noël<>", s.gsub(/\b/m) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/\b/m)
      assert_equal ["été", "\n", "noël"], s.split(/\b/m)
      assert_equal ["été", "\n", "noël", ""], s.split(/\b/m, -1)
    end

    def test_c68
      s = "été\nnoël"
      assert_equal "> té\nnoël", s.gsub(/\bé|\ba/, "> ") # ba
      assert_equal "> té\nnoël", s.sub(/\bé|\ba/, "> ")
      assert_equal "<é>té\nnoël", s.gsub(/\bé|\ba/) { |m| "<#{m}>" }
      assert_equal ["é"], s.scan(/\bé|\ba/)
      assert_equal ["", "té\nnoël"], s.split(/\bé|\ba/)
      assert_equal ["", "té\nnoël"], s.split(/\bé|\ba/, -1)
    end

    def test_c69
      s = "été\nnoël"
      assert_equal "été\nnoël", s.gsub(/^a/, "> ") # caret_a
      assert_equal "été\nnoël", s.sub(/^a/, "> ")
      assert_equal "été\nnoël", s.gsub(/^a/) { |m| "<#{m}>" }
      assert_equal [], s.scan(/^a/)
      assert_equal ["été\nnoël"], s.split(/^a/)
      assert_equal ["été\nnoël"], s.split(/^a/, -1)
    end

    def test_c70
      s = "été\nnoël"
      assert_equal "> é> t> é> \n> n> o> ë> l> ", s.gsub(/b*/, "> ") # x*
      assert_equal "> été\nnoël", s.sub(/b*/, "> ")
      assert_equal "<>é<>t<>é<>\n<>n<>o<>ë<>l<>", s.gsub(/b*/) { |m| "<#{m}>" }
      assert_equal ["", "", "", "", "", "", "", "", ""], s.scan(/b*/)
      assert_equal ["é", "t", "é", "\n", "n", "o", "ë", "l"], s.split(/b*/)
      assert_equal ["é", "t", "é", "\n", "n", "o", "ë", "l", ""], s.split(/b*/, -1)
    end

    def test_c71
      s = "été\nnoël"
      assert_equal "> été\n> noël", s.gsub(/(^)/, "> ") # (^)
      assert_equal "> été\nnoël", s.sub(/(^)/, "> ")
      assert_equal "<>été\n<>noël", s.gsub(/(^)/) { |m| "<#{m}>" }
      assert_equal [[""], [""]], s.scan(/(^)/)
      assert_equal ["été\n", "", "noël"], s.split(/(^)/)
      assert_equal ["été\n", "", "noël"], s.split(/(^)/, -1)
    end

    def test_c72
      s = "été\nnoël"
      assert_equal "> été> \n> noël> ", s.gsub(/^|$/, "> ") # ^|$
      assert_equal "> été\nnoël", s.sub(/^|$/, "> ")
      assert_equal "<>été<>\n<>noël<>", s.gsub(/^|$/) { |m| "<#{m}>" }
      assert_equal ["", "", "", ""], s.scan(/^|$/)
      assert_equal ["été", "\n", "noël"], s.split(/^|$/)
      assert_equal ["été", "\n", "noël", ""], s.split(/^|$/, -1)
    end
  end
end
