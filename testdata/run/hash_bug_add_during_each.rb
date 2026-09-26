# skip: adding a key during iteration raises RuntimeError in MRI; rb2go silently appends the key

# rbs_inline: enabled

h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
begin
  h.each do |k, v|
    puts "visit #{k}"
    h["new#{k}"] = v
  end
rescue => e
  puts e.class, e.message
end
puts h.inspect
