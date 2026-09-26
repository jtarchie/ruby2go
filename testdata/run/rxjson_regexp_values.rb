# rbs_inline: enabled

# Regexp and MatchData values in Structs, ivars, overrides, blocks and conditions.

Rule = Struct.new(:name, :re) #: [String, Regexp]

class Lexer
  attr_reader :last #: MatchData?

  #: (String) -> void
  def initialize(prefix)
    @prefix = prefix
    @last = nil
  end

  #: () -> Regexp
  def pat = /\A#{@prefix}(\d+)/

  #: (String) -> String?
  def scan(s)
    @last = pat.match(s)
    l = @last
    l ? l[1] : nil
  end
end

class Base
  #: () -> Regexp
  def re = /base/

  #: (String) -> bool
  def ok?(s) = re.match?(s)
end

class Child < Base
  #: () -> Regexp
  def re = /child/i
end

rules = [Rule.new("num", /\A\d+\z/), Rule.new("word", /\A\w+\z/)]
["12", "ab", "!"].each do |tok|
  r = rules.find { |ru| ru.re.match?(tok) }
  puts "#{tok}: #{r ? r.name : "none"}"
end
puts rules.map { |ru| ru.re.source }.inspect, rules[0].inspect
lx = Lexer.new("id")
puts lx.scan("id42x").inspect, lx.scan("x").inspect, lx.last.inspect, lx.pat.inspect
lx.scan("id7")
puts lx.last.inspect, lx.last&.pre_match.inspect
bs = [Base.new, Child.new] #: Array[Base]
puts bs.map { |b| b.ok?("CHILD") }.inspect, bs.map { |b| b.re.inspect }.inspect
pairs = { "x" => /x+/, "y" => /y/ } #: Hash[String, Regexp]
pairs.each { |k, v| puts "#{k} #{v.match?("xx")} #{("axx" =~ v).inspect}" }
puts pairs.select { |_k, v| v.match?("y") }.keys.inspect

# Assignment in a condition, `||` defaults, optional groups through blocks.
line = "name=bob"
if (m = line.match(/(\w+)=(\w+)/))
  puts m[1], m[2]
end
m2 = "x".match(/(y)?x/)
puts m2[1] || "default" if m2
m3 = "k=".match(/(\w)=(\w)?/)
puts m3.captures.map { |c| c.to_s }.inspect if m3
m3.captures.each { |c| puts c.inspect } if m3
p1 = /(\d+)/
puts ["a1", "b22", "c"].map { |s| (mm = p1.match(s)) ? mm[1].to_s : "" }.inspect
puts ["1", "x", "3"].select { |s| s.match?(/\A\d+\z/) }.map(&:to_i).inspect
cm = "ab".match(/(a)/)
if cm
  cs = cm.captures
  cs << "extra"
  puts cs.size, cm.captures.size
end

# Regexp?: nil prints, narrowing, &., and a method that may return nil.
rr = nil #: Regexp?
puts rr.inspect, rr.to_s.inspect, rr.nil?
rr = /z/
puts rr.inspect if rr
puts (rr&.match?("z")).inspect

#: (String) -> Regexp?
def pick(s) = s.empty? ? nil : /#{s}/

puts pick("").inspect, pick("q").inspect, pick("q")&.source.inspect

# Object-level behaviour: literals are frozen Regexp objects; puts uses to_s.
puts(/a/.frozen?, /a/.class, "a".match(/a/).class, /a/.is_a?(Regexp), /a/.is_a?(Object))
puts(/a/)
puts(/\n/.source, /\n/.inspect, /\t/.to_s, /"/.inspect, /\\/.source, /\\/.inspect, /é日/.inspect, /é日/.source)
puts(/a b/.inspect, /\s+/.to_s, /[a-z]\d{2,}/.source.length, /x/.inspect.size)

# Collections of matches, chained through &. and blocks.
re = /(\w+)@(\w+)\.com/
emails = ["a@b.com", "bad", "c@d.com"]
puts emails.map { |e| re.match(e) }.map { |mm| mm ? mm[1].to_s : "?" }.inspect
puts emails.select { |e| re.match?(e) }.map { |e| e.match(re)&.captures&.last.inspect }.inspect
puts emails.select { |e| e.match?(re) }.size, emails.find { |e| !e.match?(re) }.inspect
puts emails.min_by { |e| (e =~ /@/) || 99 }
puts "a-b".match(/(\w)-(\w)/)&.captures&.map { |c| c.to_s.upcase }.inspect
puts "ab".match(/(\w)-(\w)/)&.captures&.map { |c| c.to_s.upcase }.inspect
puts "k=v; x=y".match(/(\w)=(\w)/).to_s.split("=").map(&:upcase).join("|")
puts "abc".match(/b/)&.pre_match&.upcase.inspect, ("abc" =~ /c/)&.to_s.inspect

# Escaped interpolation is literal text; a newline inside a literal is part of the pattern; {x} is not a quantifier.
iq = "q"
puts(/\#{iq}/.match?('#{iq}'), /\#{iq}/.source, /a#b/.inspect, /\#{iq}#{iq}/.source)
nl = /a
b/
puts nl.match?("a\nb"), nl.source.size, nl.inspect.size
puts(/a{x}/.match?("a{x}"), /a{2,}?/.match("aaa").inspect)

# Untyped receivers and arguments through dynamic dispatch (decision 32).

#: () -> untyped
def five = 5

ux = "abc" #: untyped
puts ux !~ /b/, ux !~ /z/
ur = /a/ #: untyped
begin
  puts ur.match?(five)
rescue TypeError => e
  puts "dyn #{e.class}"
end
puts ur === "cat", ur === 5, ur == /a/, /a/ == ur, ur != /b/
