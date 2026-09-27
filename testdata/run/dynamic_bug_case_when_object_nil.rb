# rbs_inline: enabled

#: (untyped) -> String
def f(v)
  case v
  when Object then "object"
  else "not an object"
  end
end

puts f(1), f(nil), f(false)
