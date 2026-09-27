# rbs_inline: enabled

class Temp
  attr_reader :deg #: Integer

  #: (Integer) -> void
  def initialize(deg)
    @deg = deg
  end

  #: (Integer) -> Temp
  def self.of(d) = new(d)

  FREEZING = of(0)
  BOILING = new(100)
end

puts Temp::FREEZING.deg.inspect, Temp::BOILING.deg.inspect
