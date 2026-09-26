# skip: a tuple that reaches untyped stays a Tuple2 struct, not an Array: is_a?(Array)/when Array are false, puts prints its inspect, == with an equal Array is false, and nested mixed literals under an untyped expectation become tuples

# rbs_inline: enabled
#: (untyped) -> bool
def arr?(x) = x.is_a?(Array)

#: (untyped) -> String
def kind(x)
  case x
  when Array then "array #{x.size}"
  else "other"
  end
end

#: (Array[untyped]) -> Array[untyped]
def flat(xs)
  out = [] #: Array[untyped]
  xs.each do |x|
    if x.is_a?(Array)
      out.concat(flat(x))
    else
      out << x
    end
  end
  out
end

pair = [1, "a"]
puts arr?(pair), arr?([1, 2]), arr?(1), kind(pair)
puts pair
puts [1, [2, [3]]]
u = [] #: Array[untyped]
u << 1
u << "a"
puts pair == u, u == pair
puts flat([1, [2, [3, [4]]], 5]).inspect
