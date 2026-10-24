# Crystal port of benchmarks/json.rb, for the README comparison column.
# Records are JSON::Serializable structs (typed generate); parse is the generic JSON.parse -> JSON::Any, as in Ruby.
require "json"

alias Depth = Array(Int32 | Array(Int32 | Array(Int32)))

struct Meta
  include JSON::Serializable

  @[JSON::Field(emit_null: true)]
  getter n : Nil = nil
  getter depth : Depth = [1, [2, [3]]] of Int32 | Array(Int32 | Array(Int32))

  def initialize
  end
end

struct Record
  include JSON::Serializable

  getter id : Int32
  getter name : String
  getter tags : Array(String)
  getter price : Float64
  getter ok : Bool
  getter meta : Meta

  def initialize(@id, @name, @tags, @price, @ok, @meta)
  end
end

records = Array.new(5_000) do |i|
  Record.new(i, "item #{i}", ["t#{i % 7}", "t#{i % 11}"], i * 1.25, i.even?, Meta.new)
end

total = 0
10.times do
  text = records.to_json
  back = JSON.parse(text)
  total += back.size + text.bytesize
end
puts total
pretty = records.first(2).to_pretty_json
puts pretty.lines.size
puts JSON.parse(pretty).as_a.last["tags"].as_a.map(&.as_s).inspect
