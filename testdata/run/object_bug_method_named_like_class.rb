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
