require "json"

# nested objects/arrays
nested = JSON.parse(%({"a":{"b":[1,{"c":2}]},"d":[[1,2],{"e":3}]}))
p nested

# numbers: int, float, negative, exponent
p JSON.parse("42")
p JSON.parse("-17")
p JSON.parse("3.14")
p JSON.parse("-3.14")
p JSON.parse("1e10")
p JSON.parse("1.5e-3")
p JSON.parse("0")
p JSON.parse("-0")
p JSON.parse("-0.0")

# strings with escapes/unicode
escaped = <<~'JSON'.chomp
  "a\tb\nc\"d\\e"
JSON
p JSON.parse(escaped)
p JSON.parse('"café"')
p JSON.parse('"😀"')

# symbolize_names
p JSON.parse(%({"a":1,"nested":{"b":2}}), symbolize_names: true)

# malformed input
begin
  JSON.parse("{bad json")
rescue JSON::ParserError => e
  puts e.class
end

begin
  JSON.parse("")
rescue JSON::ParserError => e
  puts e.class
end

begin
  JSON.parse(%({"a":1,}))
rescue JSON::ParserError => e
  puts e.class
end

# pretty_generate
puts JSON.pretty_generate({"name" => "Ada", "langs" => ["ruby", "go"], "meta" => {"active" => true}})

# dump/load round trip
data = {"x" => 1, "y" => [1, 2, 3]}
dumped = JSON.dump(data)
puts dumped
loaded = JSON.load(dumped)
p loaded
p loaded == data

# generate options round trip
opts_out = JSON.generate({"k" => "v"}, indent: "\t", space: " ", object_nl: "\n")
puts opts_out
p JSON.parse(opts_out) == {"k" => "v"}

# generate -> parse round trip for a mixed structure
complex = {"list" => [1, 2.5, "three", true, false, nil, {"nested" => [1, 2]}]}
round = JSON.parse(JSON.generate(complex))
p round == complex
