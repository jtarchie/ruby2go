# skip: a method whose Go name equals its class's name (Calc#calc, Node#node) in a class that has a subclass fails go build "field and method with the same name Calc": the subclass struct embeds the parent as a field named Calc

# rbs_inline: enabled

class Calc
  #: (Integer) -> Integer
  def calc(n) = n * 2
end

class KidCalc < Calc
  def calc(n) = super + 1
end

class Node
  attr_reader :node #: String

  #: () -> void
  def initialize
    @node = "node"
  end
end

class Leaf < Node
end

puts Calc.new.calc(1).inspect, KidCalc.new.calc(2).inspect, Leaf.new.node
