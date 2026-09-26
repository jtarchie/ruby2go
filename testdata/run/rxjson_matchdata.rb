# rbs_inline: enabled

#: (MatchData?) -> void
def dump(m)
  unless m
    puts "no match"
    return
  end
  puts m.inspect, m.to_s.inspect, m.pre_match.inspect, m.post_match.inspect, m.captures.size
  puts "[0]=#{m[0].inspect} [1]=#{m[1].inspect} [9]=#{m[9].inspect} [-9]=#{m[-9].inspect}"
  puts "[-1]=#{m[-1].inspect}" if m.captures.size > 0
end

dump(/(\d+)-(\d+)/.match("ab 10-20 cd"))
dump("héllo wörld".match(/(ö)(r)/))
dump("日本語テキスト".match(/本(.)/))
dump("emoji 😀 ok".match(/😀 (\w+)/))
dump("".match(/^$/))
dump("abc".match(/b/))
dump("abc".match(/z/))
dump("tab\there".match(/\t(h)/))
dump("q\"x\\".match(/"(x)(\\)/))
dump("line1\nline2".match(/^(line2)$/))

# An unmatched optional group is nil, not "".
m = /(\d+)-(\d+)?(x)?/.match("ab 10- cd")
if m
  puts m.inspect, m[1].inspect, m[2].inspect, m[3].inspect, m[-1].inspect, m[-2].inspect, m[-3].inspect
  puts m.captures.size, m.captures[0].inspect
end
alt = /(a)|(b)/.match("b")
puts alt.inspect
puts alt[1].inspect, alt[2].inspect if alt

m2 = "key=value; other=x".match(/(\w+)=(\w+)/)
if m2
  puts m2
  puts "#{m2} / #{m2[1]} / #{m2[2]}"
  k, v = m2.captures
  puts k.inspect, v.inspect
  puts m2.captures.map { |c| c.to_s.upcase }.inspect
end

path = "/posts/42.json"
puts path =~ /\d+/, (path =~ /zzz/).inspect, path !~ /\.html$/, path !~ /json/
puts path.match?(/json/), "abc".match?(/^b/), "Hello".match?(/hello/i), "".match?(//)
puts path.match(/xml/).inspect
pm = path.match(%r{\A/(\w+)/(\d+)(\.\w+)?\z})
puts pm.inspect, pm[1].to_s + "#" + pm[2].to_s if pm

# Edge matches: an empty group is "", not nil; a repeated group keeps its last iteration.
puts "abc".match(/(x?)b/).inspect, "abc".match(//).inspect, "abc".match(/$/).inspect
puts "123".match(/(\d)+/).inspect, "ab".match(/((a)(b))/).inspect, "ab".match(/(?:a)(b)/).inspect
em = "abc".match(/$/)
puts em.pre_match.inspect, em.post_match.inspect if em
em2 = "abc".match(//)
puts em2.pre_match.inspect, em2.post_match.inspect if em2

# Safe navigation and boolean contexts over MatchData? and Integer?.
puts "a-b".match(/(\w)-(\w)/)&.captures&.join("+").inspect, "ab".match(/(\w)-(\w)/)&.captures&.join("+").inspect
puts "a-b".match(/(\w)-(\w)/)&.[](2).inspect, ("x9" =~ /\d/)&.succ.inspect, "a-b".match(/-/)&.pre_match.inspect
puts "ab12cd".match(/\d+/).to_s.to_i + 1, "ab".match(/\d+/).to_s.inspect
puts !("abc" =~ /z/), ("abc" =~ /z/).nil?, (("abc" =~ /c/) || -1), (("abc" =~ /z/) || -1)
puts ("abc" =~ /b/) == 1, ("abc" =~ /z/) == nil, "abc".match(/b/).nil?, "abc".match(/z/) == nil
puts ("abc".match(/z/) || "abc".match(/c/)).inspect, ("x".match(/y/) || "none").inspect
if (i = "héllo" =~ /l/)
  puts i + 1
end
while (wm = "abc".match(/c/))
  puts wm
  break
end

# MatchData as a value: returned, stored, untyped.

#: (String) -> String?
def num(s)
  m = s.match(/(\d+)/)
  return nil unless m
  m[1]
end
puts num("ab12").inspect, num("ab").inspect
ms = ["a1", "b", "c3"].map { |s| s.match(/(\w)(\d)/) }
ms.each { |mm| puts mm.inspect }
puts ms.reject { |mm| mm.nil? }.size, ms.map { |mm| mm ? mm[1] : "-" }.inspect
store = {} #: Hash[String, MatchData]
xm = "xy".match(/x(y)/)
store["m"] = xm if xm
puts store["m"].inspect, store.inspect
kv = {} #: Hash[String, String?]
"a=1\nb=2\nc".split("\n").each do |l|
  lm = l.match(/\A(\w)=(\d)\z/)
  next unless lm
  key = lm[1]
  kv[key] = lm[2] if key
end
puts kv.inspect
mu = "k=v".match(/(\w)=(\w)/) #: untyped
puts mu[1].inspect, mu.captures.inspect, mu.pre_match.inspect, mu.post_match.inspect, mu.to_s, mu.inspect, mu[-1]

# Calling a method on a failed match raises NoMethodError, as Ruby does.
begin
  puts "abc".match(/z/)[0].inspect
rescue NoMethodError => e
  puts e.class, e.message
end
begin
  puts ("abc" =~ /z/) + 1
rescue NoMethodError => e
  puts e.class, e.message
end
puts "before uncaught"
puts "abc".match(/q/).pre_match
puts "not reached"
