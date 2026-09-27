# rbs_inline: enabled

require "json"

# captures.compact drops the unmatched optional group; compact on a MatchData? array
m_compact = /(a)(b)?/.match("a")
puts m_compact.captures.compact.size if m_compact
found = ["a1", "b", "c3"].map { |s| s.match(/\d/) }.compact
puts found.size

# an unmatched group is nil in captures, destructuring, join and inspect
m_nil = /(a)(b)?/.match("a")
if m_nil
  c = m_nil.captures
  puts c.size
  puts c[1].inspect
  k, v = m_nil.captures
  puts k.inspect, v.inspect
  puts c.inspect
  puts c.join(","), m_nil.captures.join("-").size
end
puts %w[cat dog].map { |w| w =~ /a/ }.inspect

# ^ does not match after a string's final newline
puts ("a\n" =~ /^$/).inspect, ("a\n" =~ /^\z/).inspect, "a\n".match?(/\n^/)

# && class intersection and a nested class
puts "aebcd".match(/[a-z&&[^aeiou]]+/).inspect, "&]".match(/[a-z&&[^aeiou]]/).inspect
puts "b".match?(/[a[bc]]/), "b]".match(/[a[bc]]/).inspect

# an interpolated Regexp keeps its own flags
inner = /ab/i
outer = /x#{inner}y/
puts outer.inspect, outer.match?("xABy"), outer.match?("xaby"), outer.match?("xy")
dotall = /a.b/m
puts(/#{dotall}/.match?("a\nb"))

# Regexp#=== on a Symbol, directly and in case/when
puts(/b/ === :abc, /z/ === :abc)
case :abc
when /b/ then puts "symbol matched"
else puts "symbol not matched"
end

# inline (?m) groups and a scoped (?-m:)
puts "a\nb".match?(/(?m:a.b)/), "a\nb".match?(/(?m)a.b/), "A\nb".match?(/(?mi)a.b/), "xa\nb".match?(/x(?-m:a.b)/m)

# a class split across an interpolation still translates \h
x_class = "z"
puts "z".match?(/[#{x_class}\h]/), "f".match?(/[#{x_class}\h]/), "]".match?(/[#{x_class}\h]/), "z]".match(/[#{x_class}\h]/).inspect

# interpolated text is Ruby regexp syntax too
h_interp = "\\h+"
puts(/#{h_interp}/.match("zzF0").inspect)

# Float#to_json uses the shortest round-trip digits
puts 1e23.to_json, 1234567890123456.8.to_json, [1e23].to_json, JSON.generate(-1e23)
puts 5.326172664550231e-12.to_json, 8.92043287120562e+16.to_json

# invalid UTF-8 raises JSON::GeneratorError, bare and nested
begin
  puts "\xff".to_json
rescue JSON::GeneratorError => e
  puts "GeneratorError #{e.class} #{e.message}"
end
begin
  puts ["ok", "a\xffb"].to_json
rescue JSON::GeneratorError => e
  puts "in array: #{e.message}"
end

# nil inside typed containers serializes as null
a_json = [1, nil] #: Array[Integer?]
puts a_json.to_json
puts ["a", nil].to_json, [1.5, nil].to_json
h_json = { "a" => nil, "b" => 1 } #: Hash[String, Integer?]
puts h_json.to_json
t_json = [1, nil] #: [Integer, String?]
puts t_json.to_json
puts({ 1 => 2, nil => 3 }.to_json)
puts JSON.generate([nil, 2])

# MatchData#[] with negative indexes, including past the start
m_neg = "abc".match(/b/)
puts m_neg[-1].inspect if m_neg
m2_neg = /(a)/.match("a")
puts m2_neg[-1].inspect, m2_neg[-2].inspect, m2_neg[-3].inspect if m2_neg

# /i with multi-character case folds
puts "straße".match?(/STRASSE/i), "SS".match?(/ß/i), "ﬀ".match?(/FF/i)

# named groups: numbered access, captures and inspect with an unmatched group
m_named = /(?<year>\d+)-(?<mon>\d+)(?<day>-\d+)?/.match("2024-05")
if m_named
  puts m_named[1].inspect, m_named[2].inspect, m_named[3].inspect, m_named.captures.size
  puts m_named.inspect
end

# named groups make plain parens non-capturing
m_plain = /(?<a>x)(y)/.match("xy")
if m_plain
  puts m_plain.captures.size, m_plain[1].inspect, m_plain[2].inspect, m_plain.captures.join(",")
end

# nil captures answer NilClass#to_i/#to_f; =~ on a nil String?
m_nilclass = /(\d+)(?:\.(\d+))?/.match("v12")
if m_nilclass
  puts m_nilclass[1].to_i
  puts m_nilclass[2].to_i, m_nilclass[2].to_f
end
line = nil #: String?
puts (line =~ /x/).inspect

# /o interpolates once, on first evaluation
["a", "b"].each do |x|
  re = /#{x}/o
  puts re.source, re.match?("b")
end

# {,n} is an open-min quantifier
puts "aaa".match(/a{,2}/).inspect, "xbb".match(/xb{,1}/).inspect, ("{,2}" =~ /a{,2}/).inspect

# a parenthesized numeric literal as receiver
puts (-1).to_json, (0.5).to_json, (-0.5).to_json

# POSIX classes are Unicode-aware
puts "é".match(/[[:alpha:]]/).inspect, "É".match(/[[:upper:]]+/).inspect, "日本1".match(/[[:alnum:]]+/).inspect
puts ("é" =~ /[[:word:]]/).inspect, ("é" =~ /[[:lower:]]/).inspect
puts "٣".match?(/[[:digit:]]/), "　".match?(/[[:space:]]/), "x　".match?(/x[[:^space:]]/)

# \u, \u{...} and \e escapes in a pattern
puts "ab".match?(/ab/), "é".match?(/é/), "é".match?(/\u{e9}/), "日本".match?(/\u{65e5 672c}/)
puts "\e[31mred\e[0m".match(/\e\[\d+m/).inspect

# equal Regexps, literal or interpolated, are one Hash key
x_key = "a"
h_key = { /a/ => 1 } #: Hash[Regexp, Integer]
puts h_key[/a/].inspect, h_key[/#{x_key}/].inspect
res = [/a/, /#{x_key}/, /b/] #: Array[Regexp]
puts res.uniq.size

# source/inspect/to_s escape / as the literal form does
x_slash = "b/c"
puts(/a\/b/.source)
puts %r{a/b}.inspect, %r{a/b}.to_s, %r{a/b}.source
puts(/a#{x_slash}/.inspect, /a#{x_slash}/.to_s, /a#{x_slash}/.source)
w_slash = "w"
puts %r{#{w_slash}/x}.inspect, /#{w_slash}\//.source, /#{w_slash}\//.inspect

# \s includes \v
puts ("\v" =~ /\s/).inspect, "a\vb".match?(/a\sb/), "\v".match?(/\S/)

# to_json state options: space, indent, newlines, script_safe
puts({ "a" => [1] }.to_json(space: " "))
puts [1, { "a" => 2 }].to_json(indent: "  ", object_nl: "\n", array_nl: "\n", space: " ")
puts " /".to_json(script_safe: true)

# when with an untyped or optional Regexp is not a static match
ur = /x/ #: untyped
opt = /d/ #: Regexp?
["xy", "ad", "zz"].each do |s|
  r = case s
      when ur then "untyped"
      when opt then "optional"
      else "none"
      end
  puts r
end

# \b uses Unicode word characters
puts "café".match(/caf\b/).inspect, "éb".match(/\bb/).inspect, "日本 x".match(/\b本/).inspect
