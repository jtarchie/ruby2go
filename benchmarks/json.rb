# rbs_inline: enabled

# JSON generate and parse round-trips over nested hashes and arrays.
require "json"

records = Array.new(5_000) do |i|
  { "id" => i, "name" => "item #{i}", "tags" => ["t#{i % 7}", "t#{i % 11}"], "price" => i * 1.25, "ok" => i.even?, "meta" => { "n" => nil, "depth" => [1, [2, [3]]] } }
end

total = 0
10.times do
  text = JSON.generate(records)
  back = JSON.parse(text)
  total += back.size + text.bytesize
end
puts total
pretty = JSON.pretty_generate(records.first(2))
puts pretty.lines.size
puts JSON.parse(pretty).last["tags"].inspect
