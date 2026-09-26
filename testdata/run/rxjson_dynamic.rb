# rbs_inline: enabled

# Interpolated regexps compile at run time; values are inserted raw, as Ruby does.

#: (String) -> Regexp
def word(w) = /\b#{w}\b/i

#: (Regexp, String) -> String
def show(re, s) = "#{re.inspect} #{re.source.inspect} #{re.to_s.inspect} =~ #{s.inspect}: #{(re =~ s).inspect}"

dot = "."
puts show(/a#{dot}c/, "xabc"), show(/a#{dot}c/, "ac")
puts show(word("Hello"), "say hello there"), show(word("Hello"), "sayhello")
n = 42
puts show(/id=#{n}$/m, "x\nid=42"), show(/id=#{n}$/m, "id=421")
sym = :key
puts show(/#{sym}:/, "a key: b")
empty = ""
puts show(/#{empty}/, "abc"), show(/x#{empty}y/, "axy")
bs = "\\d+"
puts show(/#{bs}/, "ab12"), show(/^#{bs}\h$/, "12f")
alt = "cat|dog"
puts show(/^(#{alt})s?$/, "dogs"), show(/^(#{alt})s?$/, "cow")
first = "a"
last = "z"
puts show(/[#{first}-#{last}]+/i, "09AbZ"), show(/#{first}.#{last}/m, "a\nz")
puts(/x#{dot}/ == /x#{dot}/, /x#{dot}/ == /x./, /x#{dot}/ == /x./i, /x#{dot}/i == /x./i)

pats = ["^a", "b$", "c+"].map { |s| /#{s}/ }
puts pats.inspect, pats.map { |r| r.match?("abccc") }.inspect

# Any value interpolates through to_s: nil, Float, Array, true, untyped, calls, literals.
none = nil #: String?
puts show(/a#{none}b/, "xab"), show(/#{1.5}/, "1x5"), show(/#{[1, 2]}/, " "), show(/#{true}/, "untrue")
ub = "b+" #: untyped
ui = 7 #: untyped
puts show(/a#{ub}/, "cabbb"), show(/#{ui}/, "17")

#: (Integer) -> String
def rep(i) = "x" * i

puts show(/^#{rep(2)}$/, "xx"), show(/^#{rep(2)}$/, "xxx"), show(/#{"lit"}/, "a lit"), show(/a#{1 + 1}b/, "a2b")
wv = "w"
wre = /#{wv}/i #: Regexp
rh = { "k" => wre, "j" => /j/ } #: Hash[String, Regexp]
puts rh["k"].inspect, rh.values.map(&:source).inspect, rh.inspect, [wre, /z/].map(&:source).inspect
ny = "q"
nm = /(?<n>#{ny})(?<o>y)?/.match("aq")
puts nm[1].inspect, nm[2].inspect, nm.pre_match if nm
puts "interp named ok" if /(?<n>#{ny})/ =~ "aq"

#: (String) -> String
def try(src)
  re = /#{src}/
  "ok #{re.inspect} #{re.match?("a(b")}"
rescue RegexpError => e
  "RegexpError #{e.class} #{e.is_a?(StandardError)}"
end

puts try("a"), try("("), try("a[b"), try("\\("), try("x{2,1}")
begin
  bad = "("
  puts(/a#{bad}b/.match?("x"))
rescue StandardError => e
  puts "rescued #{e.class}"
end
unclosed = "["
puts "before uncaught"
puts(/#{unclosed}/.inspect)
puts "not reached"
