# rbs_inline: enabled

class User
  #: () -> String
  def to_s = "user"
end

class NewUser
  #: () -> String
  def to_s = "new user"
end

class Shape
  #: () -> String
  def to_s = "shape"
end

class ShapeI
  #: () -> String
  def to_s = "shape i"
end

module Foo
  class Bar
    #: () -> String
    def to_s = "Foo::Bar"
  end
end

class Foo_Bar
  #: () -> String
  def to_s = "Foo_Bar"
end

puts User.new, NewUser.new, Shape.new, ShapeI.new, Foo::Bar.new, Foo_Bar.new
puts NewUser.name, ShapeI.name, Foo_Bar.name, Foo::Bar.name
