# rbs_inline: enabled

# Loaded second: reopens a's class, replaces its helper, reuses the local's name.
class Shape
  #: () -> Integer
  def sides = 4
end

#: () -> String
def greeting = "hello from b"

LOADED << "b"
x = "b's own x" # not a's x: each file's top level has its own locals
puts "b: #{greeting}, x=#{x}, #{Shape.new.name} has #{Shape.new.sides} sides"
puts LOADED.inspect
