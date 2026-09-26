# rbs_inline: enabled
require "json"

# User to_json reached through untyped values, mixins, inheritance and nesting.

class Tag
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: (*untyped) -> String
  def to_json(*a) = { "tag" => name }.to_json(*a)
end

class SubTag < Tag
end

class Plain
  #: () -> String
  def to_s = "plain!"
end

module Hashable
  #: (*untyped) -> String
  def to_json(*a) = to_h.to_json(*a)
end

class Rec
  include Hashable

  #: () -> Hash[String, Integer]
  def to_h = { "r" => 1 }
end

u = Tag.new("a") #: untyped
puts u.to_json, JSON.generate(u), [u].to_json
up = Plain.new #: untyped
puts up.to_json, { "p" => up }.to_json
mixed = [Tag.new("x"), Plain.new, 1, nil, "s", Rec.new] #: Array[untyped]
puts mixed.to_json, JSON.generate(mixed)
puts Rec.new.to_json, [Rec.new].to_json, JSON.generate({ "rec" => Rec.new })
puts SubTag.new("s").to_json, [SubTag.new("t")].to_json, JSON.generate(SubTag.new("g"))
tags = [Tag.new("p"), SubTag.new("q")] #: Array[Tag]
puts tags.to_json, { "tags" => tags, "n" => tags.size }.to_json
puts({ tags: tags.map(&:name) }.to_json)

puts({ a: [1, { b: nil }] }.to_json, { a: 1, b: "two" }.to_json)
deep = { "x" => [[1, 2], [3]] } #: Hash[String, Array[Array[Integer]]]
puts deep.to_json, JSON.generate({ "a" => { "b" => { "c" => [true, 1.0e-5] } } })
puts [1, [2.5, ["s", [:t, [false]]]]].to_json
ih = { 1 => [1], 2 => [] } #: Hash[Integer, Array[Integer]]
puts ih.to_json, { 1e20 => 1, 100.0 => 2, -0.5 => 3 }.to_json
list = [] #: Array[String]
list << "a" << "b"
puts list.to_json, { "k" => list.map(&:upcase) }.to_json
empty = [] #: Array[Integer]
eh = {} #: Hash[String, Integer]
puts empty.to_json, eh.to_json, [empty, empty].to_json
j = [1, 2].to_json
puts "#{j}!", j.size, j.class, j + "x", 1.to_json.to_json, "x".to_json.to_json, :a.to_json.class
