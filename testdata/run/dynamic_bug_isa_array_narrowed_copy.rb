# skip: narrowing an untyped value holding a typed Array (is_a?(Array), case/when Array) goes through Array#_to_any, which copies it, so writes through the narrowed view are lost; MRI mutates the same array

# rbs_inline: enabled

#: (untyped) -> void
def push_if_array(v)
  v << 99 if v.is_a?(Array)
end

#: (untyped) -> void
def push_case(v)
  case v
  when Array then v << 7
  end
end

a = [1, 2] #: Array[Integer]
push_if_array(a)
push_case(a)
puts a.inspect
