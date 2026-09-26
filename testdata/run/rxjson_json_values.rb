# rbs_inline: enabled
require "json"

# to_json overrides in Struct/Data blocks, class objects, inherited and mixed-in to_s, untyped values.

P = Struct.new(:x, :y) do
  #: (*untyped) -> String
  def to_json(*a) = { "x" => x, "y" => y }.to_json(*a)
end #: [Integer, Integer]

D = Data.define(:name) do
  #: (*untyped) -> String
  def to_json(*a) = [name].to_json(*a)
end #: [String]

class Q
  #: () -> String
  def to_s = "say \"hi\"\n"
end

module Named
  #: () -> String
  def to_s = "named"
end

class Thing
  include Named
end

class Parent
  #: () -> String
  def to_s = "parent"
end

class Kid < Parent
end

# super from a to_json override reaches Kernel#to_json (the to_s JSON), then a subclass's super reaches that.
class Wrap
  #: () -> String
  def to_s = "w"

  #: (*untyped) -> String
  def to_json(*a) = "{\"wrapped\":#{super}}"
end

class Str < Wrap
  #: (*untyped) -> String
  def to_json(*a) = "[#{super(*a)}]"
end

puts P.new(1, 2).to_json, [P.new(3, 4)].to_json, JSON.generate({ "p" => P.new(5, 6) })
puts D.new("d").to_json, { "d" => [D.new("e")] }.to_json
puts String.to_json, JSON.generate(Q), Q.new.to_json, [Q.new].to_json
puts [Thing.new, Kid.new].to_json, JSON.generate({ "k" => Kid.new }), Kid.new.to_json
puts [1].send(:to_json), "x".public_send(:to_json)
puts Wrap.new.to_json, [Wrap.new].to_json, Str.new.to_json, JSON.generate({ "s" => [Str.new] })

fh = { "a" => 1.5, "b" => 1e20 } #: Hash[String, Float]
puts fh.to_json
fo = 2.5 #: Float?
bo = true #: bool?
puts fo.to_json, bo.to_json
sh = { a: [:x, :y], b: [] } #: Hash[Symbol, Array[Symbol]]
puts sh.to_json
mk = { 1 => 2, "a" => 3, :s => 4 }
puts mk.to_json
e = RuntimeError.new("bad \"q\"")
puts e.to_json, JSON.generate([e.message])
puts [1, [2, "x"], { "k" => nil }].to_json
puts ["x1", "y2"].map { |s| [s, s.match?(/1/)] }.to_json

#: (untyped) -> String
def enc(v) = JSON.generate(v)

puts enc(1), enc("s"), enc(nil), enc({ a: 1 }), enc(Q.new), enc(:s), enc(1.0), enc(true), enc(P.new(0, 0))
um = "ab".match(/(b)/) #: untyped
ur = /x/i #: untyped
up = P.new(7, 8) #: untyped
puts JSON.generate(um), JSON.generate(ur), JSON.generate(up), JSON.generate([um, ur, up]), up.to_json

# to_json returns a fresh String: later mutation of the source does not change it.
arr = [1, 2]
s = arr.to_json
arr << 3
puts s, arr.to_json
puts({ "k" => "v" }.to_json.length, [].to_json.empty?, JSON::GeneratorError.new("x").message)
