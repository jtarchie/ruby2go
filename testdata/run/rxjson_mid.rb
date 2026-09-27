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
