# rbs_inline: enabled

class Shape
end

class Circle < Shape
  #: () -> Integer
  def radius = 3
end

#: (Shape) -> String
def big(s)
  if s.is_a?(Circle) && s.radius > 1
    "big circle"
  else
    "other"
  end
end

#: (untyped) -> String
def long(v)
  if v.is_a?(String) && v.size > 2
    "long string"
  else
    "other"
  end
end

#: (untyped, bool) -> String
def flagged(v, flag)
  v.is_a?(String) && flag ? "yes" : "no"
end

#: (untyped, bool) -> String
def either(v, flag)
  v.is_a?(Integer) || flag ? "yes" : "no"
end

puts big(Circle.new), big(Shape.new)
puts long("abc"), long("a"), long(1)
puts flagged("s", true), flagged("s", false), flagged(1, true)
puts either(1, false), either("s", true), either("s", false)
