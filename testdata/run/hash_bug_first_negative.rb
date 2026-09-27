# rbs_inline: enabled

h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
begin
  puts h.first(-1).inspect
rescue ArgumentError => e
  puts e.message
end
