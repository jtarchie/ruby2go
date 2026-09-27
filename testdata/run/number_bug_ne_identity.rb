# rbs_inline: enabled

class Money
  attr_reader :cents #: Integer

  #: (Integer) -> void
  def initialize(cents)
    @cents = cents
  end

  #: (untyped) -> bool
  def ==(other) = other.is_a?(Money) && other.cents == cents
end

x = 1 #: Integer
y = 1.0 #: Float
puts (x != 1.0).inspect, (y != 1).inspect, (x != y).inspect, (y != x).inspect
puts (Money.new(5) == Money.new(5)).inspect, (Money.new(5) != Money.new(5)).inspect
