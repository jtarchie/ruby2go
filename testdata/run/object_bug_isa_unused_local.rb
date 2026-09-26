# skip: a local whose only use is an is_a? that the static types decide is dropped from the output, so go build fails "declared and not used: d" (same fold as dynamic_bug_isa_const_temp)

# rbs_inline: enabled

class Animal
end

class Dog < Animal
end

d = Dog.new
puts d.is_a?(Animal).inspect
