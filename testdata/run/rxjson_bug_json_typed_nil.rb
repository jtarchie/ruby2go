# skip: to_json panics on nil elements of typed optional containers (Array[Integer?], Hash[String, Integer?], tuples, nil keys); MRI emits null

# rbs_inline: enabled

require "json"

a = [1, nil] #: Array[Integer?]
puts a.to_json
puts ["a", nil].to_json, [1.5, nil].to_json
h = { "a" => nil, "b" => 1 } #: Hash[String, Integer?]
puts h.to_json
t = [1, nil] #: [Integer, String?]
puts t.to_json
puts({ 1 => 2, nil => 3 }.to_json)
puts JSON.generate([nil, 2])
