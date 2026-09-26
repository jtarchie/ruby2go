# skip: a scoped constant path (Child::COLOR, Child::Nested) only looks in Child itself, so a constant inherited from the superclass is "uninitialized constant"; MRI searches Child's ancestors

# rbs_inline: enabled

class Parent
  COLOR = "red"

  class Nested
    #: () -> String
    def hi = "nested"
  end
end

class Child < Parent
  SIZE = 5
end

puts Child::SIZE.inspect
puts Child::COLOR
puts Child::Nested.new.hi
