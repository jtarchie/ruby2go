# frozen_string_literal: true

# rbs_inline: enabled

# Frozen literals (README "The prelude idea") make identity checks below deterministic.

# <=> is a byte comparison returning -1/0/1; shorter prefix sorts first.
puts ("a" <=> "b").inspect, ("b" <=> "a").inspect, ("a" <=> "a").inspect
puts ("a" <=> "ab").inspect, ("" <=> "a").inspect, ("B" <=> "a").inspect, ("é" <=> "z").inspect

# == takes anything; other classes are never equal.
puts ("a" == "a").inspect, ("a" == "b").inspect, ("a" == :a).inspect, ("1" == 1).inspect, ("" == nil).inspect
puts ("a" != "b").inspect, ("a" != "a").inspect, (!"a").inspect, (!"").inspect
u = "a" #: untyped
puts ("a" == u).inspect, (:a == u).inspect, ("b" == u).inspect

# + and *
puts ("ab" + "cd").inspect, ("" + "").inspect, ("é" + "😀").inspect
puts ("ab" * 3).inspect, ("ab" * 0).inspect, ("" * 5).inspect, ("é" * 2).inspect

# to_s/to_str/dup return equal strings
puts "abc".to_s.inspect, "abc".to_str.inspect, "abc".dup.inspect

# size counts characters, bytesize bytes
puts "".size.inspect, "héllo".size.inspect, "héllo".length.inspect, "😀".size.inspect
puts "héllo".bytesize.inspect, "😀".bytesize.inspect, "".bytesize.inspect
puts "".empty?.inspect, " ".empty?.inspect, "\n".empty?.inspect

# prefix/suffix/substring predicates, including the empty needle
puts "hello".start_with?("he").inspect, "hello".start_with?("").inspect, "hello".start_with?("lo").inspect, "".start_with?("a").inspect
puts "hello".end_with?("lo").inspect, "hello".end_with?("").inspect, "héllo".end_with?("éllo").inspect, "lo".end_with?("hello").inspect
puts "hello".include?("ll").inspect, "hello".include?("").inspect, "hello".include?("L").inspect, "日本語".include?("本").inspect

# index is a character offset (not a byte offset) or nil
puts "héllo".index("l").inspect, "hello".index("").inspect, "hello".index("z").inspect
puts "日本語".index("語").inspect, "".index("").inspect, "abab".index("b").inspect

# [] with one Integer: negative counts from the end, out of range is nil
puts "héllo"[1].inspect, "abc"[0].inspect, "abc"[-1].inspect, "abc"[-3].inspect
puts "abc"[-4].inspect, "abc"[3].inspect, ""[0].inspect, "日本"[-1].inspect
first = "xyz"[0]
puts first.upcase if first
puts ("abc"[1] == "b").inspect, "abc"[7].nil?.inspect

# Comparable on String
puts ("a" < "b").inspect, ("a" <= "a").inspect, ("b" > "a").inspect, ("a" >= "b").inspect, ("Z" < "a").inspect
puts "m".between?("a", "z").inspect, "m".between?("m", "m").inspect, "A".between?("a", "z").inspect
puts "m".clamp("a", "f").inspect, "b".clamp("c", "f").inspect, "d".clamp("c", "f").inspect, "c".clamp("c", "c").inspect

# hash is only comparable within a run
puts ("abc".hash == "abc".hash).inspect, ("abc".hash == "abd".hash).inspect, ("a".hash == "a".dup.hash).inspect
puts ("ab" + "c").hash == "abc".hash

# identity: literals are frozen and shared, dup and + make new strings
t = "abc"
puts t.equal?(t).inspect, t.equal?(t.dup).inspect, (t.dup == t).inspect, "abc".equal?("abc").inspect
puts ("a" + "b").equal?("a" + "b").inspect, ("a" + "b") == "ab"
puts "lit".frozen?.inspect, "x".freeze.inspect, "x".freeze.equal?("x").inspect
puts "hi".nil?.inspect

# Kernel#then on a String
puts "hi".then { |x| x + "!" }.inspect, "3".then { |x| x.to_i + 1 }.inspect

# sorting and min/max go through <=>
puts ["b", "a", "C", "", "aa"].sort.inspect, ["b", "a"].max.inspect, ["b", "a"].min.inspect

# strings as hash keys compare by value
counts = {} #: Hash[String, Integer]
["x", "y", "x", "x" + ""].each { |k| counts[k] = (counts[k] || 0) + 1 }
puts counts.inspect

# case/when compares strings by value
["hi", "ho", "h" + "i", "?"].each do |w|
  case w
  when "ho" then puts "ho!"
  when "hi" then puts "hi!"
  else puts "other"
  end
end

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

w = Word.new("hello")
puts w[1].inspect, w.index("l").inspect, (w =~ "ell").inspect, w.match("hello").inspect, w.match("ell").inspect
puts (w <=> Word.new("world")).inspect, (-w).s, (+w).s, (!w).inspect, (!Word.new("")).inspect, w.idx_set(2)
