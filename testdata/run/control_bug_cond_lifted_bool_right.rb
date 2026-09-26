# skip: an if/unless condition `a && b` or `a || b` whose right side needs lifted statements and ends in a method returning bool (e.g. `(h[k] || 0) > 1`) stores Boolean into a Go bool temp, so go build fails (cannot use ... as bool value)

# rbs_inline: enabled

h = { "a" => 1, "b" => 5 } #: Hash[String, Integer]
if h["c"].nil? && (h["b"] || 0) > 1
  puts "lifted and"
end
if h["a"].nil? || (h["b"] || 0) > 1
  puts "lifted or"
end
ok = true
unless ok && (h["zz"] || 0) > 1
  puts "unless lifted and"
end
