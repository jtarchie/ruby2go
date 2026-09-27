# rbs_inline: enabled

h = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
v = h["a"]
puts v.nil?.inspect
puts h["a"].nil?.inspect, h["zz"].nil?.inspect, h["b"].nil?.inspect
if h["a"]
  puts "truthy"
else
  puts "falsy"
end
puts((h["a"] || 7).to_s)
inferred = { "x" => nil, "y" => 1 }
puts inferred["x"].nil?.inspect
d = h.delete("a")
puts d.nil?.inspect, h.size
puts h["a"].inspect
h["c"] = nil
puts h.delete("c").inspect
