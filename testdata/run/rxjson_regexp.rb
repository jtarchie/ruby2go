# rbs_inline: enabled

# Decision 24: Ruby regexp syntax and semantics on Go's RE2.

DIGITS = /\d+/ #: Regexp

class Finder
  attr_reader :re #: Regexp

  #: (Regexp) -> void
  def initialize(re)
    @re = re
  end

  #: (String) -> Integer?
  def at(s) = s =~ re
end

#: (Regexp, String) -> String
def show(re, s) = "#{re.inspect} =~ #{s.inspect}: #{(re =~ s).inspect}"

[/(\d+)-(\d+)/, /abc/i, /a.c/m, /a.c/mi, /x/, //, /tab\there/].each do |re|
  puts "#{re.inspect} #{re.source.inspect} #{re.to_s.inspect}"
end
puts(/a/im.inspect)
puts "#{/y/m}|#{/z/}"
puts %r{\d+}.inspect, %r{\d+}.source

puts(/ab/ == /ab/, /ab/ == /ab/i, /ab/i == /ab/i, /ab/m == /ab/i, /ab/ == /abc/)
puts(/ab/ != /ab/, /ab/ == "ab")
puts [/a/, /b/i].include?(/b/i), [/a/, /b/i].include?(/b/)

puts(/(\d+)-(\d+)/.match?("10-20"), /(\d+)-(\d+)/.match?("10-"), /x/.match?(""), //.match?(""))
puts show(/\d+/, "ab 10-20"), show(/zzz/, "abc"), show(//, ""), show(/$/, "abc")
puts show(/l/, "héllo"), show(/w/, "héllo wörld"), show(/語/, "日本語"), show(/!/, "😀😀!")
puts DIGITS.match?("x9"), (DIGITS =~ "ab12").inspect
f = Finder.new(/o+/)
puts f.at("foo").inspect, f.at("bar").inspect, f.re.source

puts show(/\h+/, "zz 0fA9"), show(/[\h]+/, "zz 0fA9"), show(/[x\h]/, "--x"), show(/\H/, "0fz9")
# Ruby's ^ and $ are always line anchors, so every pattern gets Go's (?m).
puts show(/^b$/, "a\nb\nc"), show(/\Ab/, "a\nb"), show(/c$/, "abc\n"), show(/c\z/, "abc\n"), show(/\Aabc\z/, "abc")
puts show(/^$/, "a\n\nb")

# Ruby's /m is dot-all: Go's (?s), not (?m).
puts show(/a.b/, "a\nb"), show(/a.b/m, "a\nb"), show(/HELLO/i, "say hello"), show(/a.B/mi, "A\nb")
puts show(/^.$/, "é"), show(/(?i)x/, "aX"), show(/(?i:y)z/, "Yz"), show(/(?i:y)z/, "YZ")

puts show(/a{2,3}/, "caaaa"), show(/a*?b/, "aab"), show(/(?:ab)+/, "xabab"), show(/[^a-z]/, "abc1")
puts show(/\p{L}+/, "12日本"), show(/\P{L}/, "é1"), show(/\x41/, "zA"), show(/\101/, "zA"), show(/\./, "a.c")
puts show(/[[:digit:]]+/, "ab42"), show(/[[:space:]]/, "a b"), show(/\bb/, "a b"), show(/\Bb/, "ab")
puts show(/a|ab/, "ab"), show(/\w+/, "  a1_ "), show(/\W/, "ab!"), show(/\S\s+/, "a \t")

puts(/a/ === "cat", /a/ === "dog", /a/ === 1, /a/ === nil, /1/ === 1)

#: (String) -> String
def kind(s)
  case s
  when /\A\d+\z/ then "int"
  when /\A\d+\.\d+\z/, /\A\.\d+\z/ then "float"
  when "yes", /\Ano\z/i then "bool"
  when /^#/ then "comment"
  when // then "other"
  else "unreachable"
  end
end

["12", "1.5", ".5", "yes", "NO", "x\n#c", "", "ab"].each { |s| puts "#{s.inspect} #{kind(s)}" }
v = 5 #: untyped
case v
when /5/ then puts "untyped int matched"
else puts "untyped int not matched"
end
w = "abc" #: untyped
case w
when /b/ then puts "untyped string matched"
else puts "untyped string not matched"
end
n = nil #: String?
case n
when /x/ then puts "nil matched"
else puts "nil not matched"
end

res = [/a/, /b/i, /c/m] #: Array[Regexp]
puts res.inspect, res.to_s, res.map { |r| r.source }.inspect
puts res.map { |r| r.match?("ABC") }.inspect, res.select { |r| r.match?("bab") }.size
puts "if-match" if "cat" =~ /a/
puts "unless-match" unless "cat" =~ /z/

# Untyped receivers reach the same methods through generated dynamic dispatch (decision 32).
ux = "abc" #: untyped
puts (ux =~ /b/).inspect, ux.match?(/c/), ux.match(/(b)(c)/).inspect, ux.match(/(b)(c)/)[1].inspect
ur = /q/i #: untyped
puts ur.match?("Q"), ur.source, (ur =~ "aq").inspect, ur.inspect, ur.to_s

# /i folds non-ASCII letters one rune at a time, as Onigmo does.
puts show(/é/i, "É"), show(/àb/i, "xÀB"), show(/σ/i, "Σ"), show(/ÿ/i, "y")
# Escaped punctuation and control escapes are literal characters in both engines.
puts(/a\-b/.match?("a-b"), /a\#b/.match?("a#b"), /\//.match?("/"), /\:/.match?(":"), /a\_b/.match?("a_b"))
puts(/a\ b/.match?("a b"), /\<b\>/.match?("<b>"), /\%\@\~\&\=\,\;\!\`/.match?("%@~&=,;!`"), /a\"b\'c/.match?("a\"b'c"))
puts(/\n/.match?("\n"), /\t/.match?("\t"), /\0/.match?("\0"), /\a/.match?("\a"), /\x1b\[\d+m/.match?("\e[31m"), /\033/.match?("\e"))
puts(/[\]]/.match?("]"), /[\[]/.match?("["), /[a\-z]/.match?("-"), /[a\-z]/.match?("b"), /[\\]/.match?("\\"), /[.]/.match?("a"))
puts show(/a{2}/, "caaa"), show(/a{2,}/, "a aaa"), show(/(a|b)*c/, "xababc"), show(/a+?/, "aaa"), show(/a??b/, "ab")
puts %r{a}i.inspect, %r[x/y].source, %r!q!.inspect, %r{\d+}m.to_s
# \d, \w and \s are ASCII-only in Ruby, as in RE2.
puts(/\d/.match?("٣"), /\w/.match?("é"), /\s/.match?("\u00a0"), /[[:digit:]]/.match?("7"), /[[:punct:]]/.match?("!"))
puts(/./.match?("\n"), /[^a]/.match?("\n"), /\A\z/.match?(""), /\A$/.match?("\n"))
