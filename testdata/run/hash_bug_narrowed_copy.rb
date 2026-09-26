# skip: narrowing an untyped value to Hash (is_a?(Hash), case/when Hash) goes through Hash#_to_any, which copies a typed hash, so writes through the narrowed view are lost

# rbs_inline: enabled

#: (untyped) -> void
def add_key(x)
  if x.is_a?(Hash)
    x["new"] = 1
  end
end

#: (untyped) -> Integer
def grow(x)
  case x
  when Hash
    x["grown"] = 2
    x.size
  else
    -1
  end
end

h = { "a" => 1 } #: Hash[String, Integer]
add_key(h)
puts h.inspect
puts grow(h), h.size
