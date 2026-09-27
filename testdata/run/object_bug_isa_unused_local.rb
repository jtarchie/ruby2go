# rbs_inline: enabled

class Animal
end

class Dog < Animal
end

d = Dog.new
puts d.is_a?(Animal).inspect
