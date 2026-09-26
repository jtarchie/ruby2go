# skip: `case v when Object` on untyped nil falls to else (Go `case any:` misses a nil interface); MRI matches nil as an Object

# rbs_inline: enabled

#: (untyped) -> String
def f(v)
  case v
  when Object then "object"
  else "not an object"
  end
end

puts f(1), f(nil), f(false)
