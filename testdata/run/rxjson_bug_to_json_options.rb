# skip: to_json(opts) ignores json generator options (space:, indent:, object_nl:, script_safe:); MRI formats with them. JSON.generate(obj, opts) is rejected, to_json(opts) compiles and prints compact JSON

# rbs_inline: enabled

require "json"

puts({ "a" => [1] }.to_json(space: " "))
puts [1, { "a" => 2 }].to_json(indent: "  ", object_nl: "\n", array_nl: "\n", space: " ")
puts "\u2028/".to_json(script_safe: true)
