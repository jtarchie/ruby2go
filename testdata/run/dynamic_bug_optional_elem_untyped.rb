# skip: a nil element of an Array[T?] seen through untyped (is_a?(Array) / when Array narrowing, a dynamic [] call) is a typed nil *T, so nil? is false; MRI says true

# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

#: (untyped) -> String
def nils(v)
  if v.is_a?(Array)
    v.map { |e| e.nil? }.inspect
  else
    "?"
  end
end

#: (untyped) -> String
def nils_case(v)
  case v
  when Array then v.select { |e| e.nil? }.size.to_s
  else "?"
  end
end

puts nils([1, nil, 3])
puts nils_case(["a", nil])
puts ident([1, nil])[1].nil?
