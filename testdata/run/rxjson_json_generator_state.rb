# rbs_inline: enabled

require "json"

# Generator state: options reach nested and user to_json, which get the state as MRI passes it.

class Tag
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: (*untyped) -> String
  def to_json(*a) = { "tag" => name, "kids" => [1, 2] }.to_json(*a)
end

class Legacy
  #: (?untyped) -> String
  def to_json(state = nil) = [state.nil?, 1].to_json(state)
end

pretty = { indent: "  ", space: " ", object_nl: "\n", array_nl: "\n" }
puts({ "t" => Tag.new("a"), "e" => [], "h" => {} }.to_json(pretty))
puts [Tag.new("b"), Legacy.new].to_json(indent: "\t", array_nl: "\n")
puts({ "a" => 1, "b" => [1, 2] }.to_json(indent: "  "))
puts({ "a" => 1 }.to_json(space_before: " ", space: " "))
puts [1].to_json(depth: 1, indent: "  ", array_nl: "\n")
puts "😀é/ ".to_json(ascii_only: true), "é/".to_json(escape_slash: true)
puts [0.0 / 0, -1.0 / 0, 1.5].to_json(allow_nan: true)
puts [1].to_json("array_nl" => "\n"), [1].to_json(array_nl: nil)
puts :sym.to_json(script_safe: true), { "/" => "/" }.to_json(script_safe: true)
u = [1, { "k" => "v" }] #: untyped
puts u.to_json(array_nl: "\n")
o = { "x" => 1 } #: Hash[String, Integer]?
puts o.to_json(space: " ")
puts [1, "two"].to_json(space: " ", array_nl: " ")
puts JSON.generate([Legacy.new])
begin
  [1].to_json(indent: 5)
rescue TypeError => e
  puts e.message
end
begin
  [1.0 / 0].to_json
rescue JSON::GeneratorError => e
  puts e.message
end
