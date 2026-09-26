# rbs_inline: enabled

# Regexps as values: constants, mixed-in and overridden methods, defaults, blocks, collections, untyped.

module Validates
  #: () -> bool
  def valid? = pattern.match?(value)

  #: () -> String?
  def first_group
    m = pattern.match(value)
    m ? m[1] : nil
  end
end

class Email
  include Validates
  PATTERN = /\A([^@\s]+)@([^@\s]+)\z/ #: Regexp

  attr_reader :value #: String

  #: (String) -> void
  def initialize(value)
    @value = value
  end

  #: () -> Regexp
  def pattern = PATTERN
end

class Loose < Email
  #: () -> Regexp
  def pattern = /@/
end

#: (String, ?Regexp) -> bool
def ok?(s, re = /\S/) = re.match?(s)

es = [Email.new("a@b.c"), Email.new("bad"), Loose.new("x@@y")] #: Array[Email]
es.each { |e| puts "#{e.value} #{e.valid?} #{e.first_group.inspect}" }
puts ok?(" "), ok?("x"), ok?("x", /y/)
puts Email::PATTERN.source, Email::PATTERN.match?("q@r")

res = [1, 2].map { |i| /#{i}+/ }
puts res.inspect, res.map { |r| r.match?("22") }.inspect

words = %w[apple Banana cherry avocado]
puts words.select { |w| w.match?(/\Aa/i) }.inspect, words.reject { |w| w.match?(/an/) }.inspect
puts words.select { |w| w.match?(/e/) }.size, words.find { |w| w.match?(/rr/) }.inspect
puts words.sort_by { |w| w.match?(/\A[A-Z]/) ? 0 : 1 }.inspect
puts words.group_by { |w| w.match?(/an/) }.inspect
puts words.select { |w| /a/ === w }.inspect
puts words.any? { |w| w.match?(/z/) }, words.all? { |w| w.match?(/\w/) }
puts words.map { |w| w.match(/(.)\z/).to_s }.join

# `when` with a Regexp held in a constant or a local is still Regexp#===.
DIG = /\d/ #: Regexp
re = /b/
["abc", "a1", "zz"].each do |s|
  r = case s
      when DIG then "const"
      when re then "local"
      else "none"
      end
  puts r
end
puts(re === "b", DIG === "5", re.===("x"))

pats = { /\A\d+\z/ => "int", /\A[a-z]+\z/ => "word" } #: Hash[Regexp, String]
["12", "ab", "?"].each do |s|
  hit = pats.find { |re2, v| re2.match?(s) && v != "" }
  puts hit ? hit[1] : "none"
end
puts pats.keys.map(&:source).inspect, pats.inspect

# Untyped values reach the typed Regexp methods by assertion (decision 32).

#: () -> untyped
def untyped_str = "cat"

puts(/a/.match?(untyped_str), (/a/ =~ untyped_str).inspect, "a cat!".match?(/#{untyped_str}!/), /(a)/.match(untyped_str).inspect)
v = untyped_str
puts(/t\z/.match(v).inspect)
h = { "re" => /t$/, "s" => "cat" } #: Hash[String, untyped]
hre = h["re"]
puts hre.match?("cat"), hre.inspect, h["s"] =~ /t/, (h["s"] =~ hre).inspect
