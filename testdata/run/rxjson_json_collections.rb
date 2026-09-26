# rbs_inline: enabled
require "json"

P = Struct.new(:x, :y) #: [Integer, Integer]
D = Data.define(:a) #: [String]

class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: () -> String
  def to_s = "Pt(#{x})"
end

# The json gem passes a generator state to nested to_json calls, hence *untyped.
class Post
  attr_reader :id #: Integer?
  attr_reader :tags #: Array[String]

  #: (Integer?, Array[String]) -> void
  def initialize(id, tags)
    @id = id
    @tags = tags
  end

  #: (*untyped) -> String
  def to_json(*_state) = { "id" => id, "tags" => tags, "pt" => Pt.new(1) }.to_json
end

class Special < Post
  #: (*untyped) -> String
  def to_json(*state) = "{\"special\":#{super}}"
end

puts [].to_json, [[]].to_json, [1, [2, [3, []]]].to_json, [1, 2, 3].to_json, ["a", "b\"c"].to_json
puts [true, false].to_json, [:a, :b].to_json, [1.5, 2.0].to_json, [nil].to_json, [nil, nil].to_json
puts({}.to_json)
puts({ "a" => {} }.to_json)
puts({ "z" => 1, "a" => 2, "m" => [3], "é/ü" => "日本" }.to_json)
puts({ name: "sym", "é" => "ünï", nested: { deep: [true, { "k" => :v }] } }.to_json)
puts({ 1 => "one", -2 => "neg", 1.5 => "float", true => "t", false => "f" }.to_json)
puts({ [1, 2] => 3, :s => 4, "x\ny" => 5 }.to_json)
puts({ Pt.new(9) => "user key" }.to_json)
h = {} #: Hash[String, Integer]
h["b"] = 2
h["a"] = 1
h["b"] = 3
puts h.to_json
h.delete("b")
puts h.to_json, JSON.generate(h)

puts Pt.new(3).to_json, [Pt.new(1), Pt.new(2)].to_json, JSON.generate({ "p" => Pt.new(4) })
puts Post.new(1, ["a"]).to_json, Post.new(nil, []).to_json
puts [Post.new(2, ["x", "y"])].to_json, JSON.generate({ "posts" => [Post.new(3, [])] })
puts Special.new(4, ["s"]).to_json, [Special.new(5, [])].to_json
posts = [Post.new(6, []), Special.new(7, [])] #: Array[Post]
puts posts.to_json

puts P.new(1, 2).to_json, [P.new(3, 4)].to_json, D.new("q").to_json, JSON.generate(D.new("r"))
puts StandardError.new("boom").to_json, [ArgumentError.new("bad")].to_json
md = "k=v".match(/(\w)=(\w)/)
puts(/a.b/mi.to_json, [/x/, /"q"/].to_json, md.to_json, JSON.generate({ "re" => /\d/ }))

pair = [1, "a"]
puts pair.to_json, JSON.generate(pair)
triple = [2, "s", 1.5]
puts triple.to_json
rows = [[1, "a"], [2, "b"]]
puts rows.to_json, { "rows" => rows }.to_json

u = [1, "two", nil, 3.0, [nil], { "k" => nil }, :sym, false] #: untyped
puts u.to_json, JSON.generate(u)
un = nil #: untyped
puts un.to_json, JSON.generate(un)
hu = { "a" => nil, "b" => 1, "c" => "s" } #: Hash[String, untyped]
puts hu.to_json
x = nil #: Integer?
y = 5 #: Integer?
s = "q" #: String?
puts x.to_json, y.to_json, s.to_json, JSON.generate(x), JSON.generate(y)

puts [1, 2, 3].map { |i| i * 2 }.to_json, ["b", "a"].sort.to_json, { "k" => [1, 2].map { |i| i.to_s } }.to_json
puts [1].to_json(nil), "s".to_json(1, 2)
