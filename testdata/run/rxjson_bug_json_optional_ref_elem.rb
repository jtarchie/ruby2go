# skip: to_json of a container of T? where T is a reference type (Array, Hash, Regexp, MatchData, a class) prints the to_s fallback "#<Array>"/"#<MatchData>" for every element, nil included

# rbs_inline: enabled

require "json"

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
puts ["a1", "b"].map { |w| w.match(/\d/) }.to_json
