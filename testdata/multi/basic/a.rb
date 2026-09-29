# rbs_inline: enabled

# Loaded first: defines a class, a helper and a constant, and runs top-level code.
class Shape
  #: () -> String
  def name = "shape"

  #: () -> Integer
  def sides = 0
end

#: () -> String
def greeting = "hello from a"

LOADED = ["a"] #: Array[String]

x = 1 # a top-level local: file-scoped in Ruby
puts "a: x=#{x}, #{Shape.new.name}" # not greeting: b redefines it, and rb2go's defs all exist before any file runs (decision 84)
