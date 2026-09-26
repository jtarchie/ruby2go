# skip: a singleton(C) value held in a variable, array or block param cannot be passed where Class or Module is expected: go build fails "Shape_MetaI does not implement ClassI (missing method _ClassOf)"; a class constant passes fine

# rbs_inline: enabled

class Shape
end

class Square < Shape
end

#: (Class) -> String
def cname(k) = k.name

#: (Module) -> String
def mname(k) = k.name

ks = [Square, Shape] #: Array[singleton(Shape)]
puts cname(Square), mname(Square)
puts cname(ks.fetch(0)), mname(ks.fetch(1))
counts = {} #: Hash[Class, Integer]
[Square, Shape, Square].each { |k| counts[k] = counts.fetch(k, 0) + 1 }
puts counts.inspect
