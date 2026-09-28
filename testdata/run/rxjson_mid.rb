# rbs_inline: enabled

require "json"

# #{} in a regexp literal is evaluated exactly once
class Ctr
  #: () -> void
  def initialize
    @n = 0
  end

  #: () -> String
  def nxt
    @n += 1
    puts "nxt called"
    @n.to_s
  end
end

ctr = Ctr.new
interp_re = /x#{ctr.nxt}/
puts interp_re.source, interp_re.match?("x1"), ctr.nxt

# to_json on optional elements and refs: nil stays null, user to_s/to_json used
class Pt
  #: () -> String
  def to_s = "pt"
end

class J
  #: (*untyped) -> String
  def to_json(*_a) = "{\"j\":1}"
end

puts [[1], nil].to_json
puts({ "a" => [1], "b" => nil }.to_json)
puts({ "a" => { "x" => 1 }, "b" => nil }.to_json)
puts [Pt.new, nil].to_json
js = [J.new] #: Array[J?]
puts js.to_json, JSON.generate({ "j" => js })
rs = [/a/] #: Array[Regexp?]
puts rs.to_json
puts({ "m" => "ab".match(/(a)/) }.to_json)
puts ["a1", "b"].map { |word| word.match(/\d/) }.to_json

# matching against invalid UTF-8 raises ArgumentError like MRI
bad_utf8 = "a\xffb"
begin
  puts bad_utf8.match?(/b/)
rescue ArgumentError => utf_err
  puts "match? #{utf_err.class}"
end
begin
  puts (bad_utf8 =~ /b/).inspect
rescue ArgumentError => utf_err
  puts "=~ #{utf_err.class}"
end
begin
  puts bad_utf8.match(/(b)/).inspect
rescue ArgumentError => utf_err
  puts "match #{utf_err.class}"
end

# an untyped Integer argument to match raises TypeError, not a Go panic

#: () -> untyped
def five = 5

begin
  puts(/a/.match?(five))
rescue TypeError => type_err
  puts "match? #{type_err.class}"
end
begin
  puts(/a/ =~ five)
rescue TypeError => type_err
  puts "=~ #{type_err.class}"
end
begin
  puts "abc".match?(five)
rescue TypeError => type_err
  puts "String#match? #{type_err.class}"
end

# untyped nil/Symbol subjects through dynamic and typed regexps

#: () -> untyped
def nothing = nil

#: () -> untyped
def sym = :cat

ur = /a/ #: untyped
begin
  puts ur.match?(nothing), (ur =~ nothing).inspect, ur.match(nothing).inspect
rescue StandardError => subj_err
  puts "dynamic nil: #{subj_err.class}"
end
begin
  puts(/a/.match?(nothing), (/a/ =~ nothing).inspect, /a/.match(nothing).inspect)
rescue StandardError => subj_err
  puts "typed nil: #{subj_err.class}"
end
begin
  puts(/a/.match?(sym), (/a/ =~ sym).inspect, /(a)/.match(sym).inspect, "cat" =~ /#{sym}/)
rescue StandardError => subj_err
  puts "symbol: #{subj_err.class}"
end

p "a1b22c333".gsub(/\d+/, "#"), "a1b2".sub(/\d/, "<\\0>"), "John Smith".sub(/(\w+) (\w+)/, "\\2, \\1")
p "a1b2".gsub(/\d/) { |d| (d.to_i * 2).to_s }, "hello world".gsub(/o/) { |m| m.upcase }, "x-y".gsub("-") { "+" }
p "2024-03-05".sub(/(?<y>\d+)-(?<m>\d+)-(?<d>\d+)/, "\\k<d>/\\k<m>/\\k<y>"), "a.b".gsub(".", "!"), "path/to".gsub("/", "\\\\")
p "abc".gsub(/x*/, "-"), "aaa".gsub(/a/, "\\\\"), "abc".sub(/b/, "[\\`|\\']"), "abc".gsub(/(b)|(z)/, "<\\2>")
p "a1b22c333".scan(/\d+/), "k1=v1; k2=v2".scan(/(\w+)=(\w+)/), "ab".scan(/(a)|(b)/), "none".scan(/\d/)
p "a, b,c ,d".split(/\s*,\s*/), "a1b2c3".split(/\d/), "abc".split(//), "a-b_c".split(/([-_])/), "1,2,,".split(/,/), ",a".split(/,/)
p "one  two".split(/ /), "camelCaseString".gsub(/([A-Z])/) { |m| "_" + m.downcase }
p 1234567.to_s.reverse.scan(/\d{1,3}/).join(",").reverse
