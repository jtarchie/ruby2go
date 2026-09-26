# skip: a method call on a parenthesized numeric literal, (-1).to_json, emits an untyped Go constant receiver and fails go build

# rbs_inline: enabled

require "json"

puts (-1).to_json, (0.5).to_json, (-0.5).to_json
