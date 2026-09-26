# skip: a dynamic call passing Array[Integer] / a Hash literal into an Array[untyped] / Hash[Symbol, untyped] (or untyped array into Array[Integer]) parameter raises TypeError "no implicit conversion of an instance of Array into Array[untyped]"; MRI passes it

# rbs_inline: enabled

class Foo
  #: (Array[untyped]) -> Integer
  def count_all(a) = a.size

  #: (Hash[Symbol, untyped]) -> Integer
  def opts(h) = h.size

  #: (Array[Integer]) -> Integer
  def total(a) = a.reduce(0) { |s, x| s + x }
end

#: (untyped) -> untyped
def ident(v) = v

o = ident(Foo.new)
nums = [1, 2] #: Array[Integer]
mixed = [1, 2] #: Array[untyped]
puts o.total(nums), o.count_all(mixed)
puts o.count_all(nums)
puts o.opts({ k: 1 })
puts o.total(mixed)
puts o.total([])
