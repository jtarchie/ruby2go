# rbs_inline: enabled

class Base
  #: () -> String
  def name = "Base"
end

class Leaf < Base
  def name = "Leaf"
end

a = Base.new #: Base
a = Leaf.new
puts a.name

x = Base.new
x = Leaf.new
puts x.name

y = Leaf.new #: Base
puts y.name
y = Base.new
puts y.name

k = Leaf #: singleton(Base)
puts k.name
k = Base
puts k.name
