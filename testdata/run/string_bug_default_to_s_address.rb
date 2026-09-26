# skip: the default Kernel#to_s/inspect is "#<Foo>"; MRI prints "#<Foo:0x...>" with the object's address

# rbs_inline: enabled

class Foo
end

f = Foo.new
puts f.to_s.start_with?("#<Foo:0x").inspect, f.to_s.end_with?(">").inspect, "#{f}".include?(":0x").inspect
puts f.inspect.start_with?("#<Foo:0x").inspect
