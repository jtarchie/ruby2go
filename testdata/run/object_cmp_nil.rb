# rbs_inline: enabled

# A <=> that may answer nil (-> Integer?) still sorts; nil raises MRI's ArgumentError, also for a subclass.

class Score
  attr_reader :v #: Float

  #: (Float) -> void
  def initialize(v)
    @v = v
  end

  #: (Score) -> Integer?
  def <=>(other) = v <=> other.v

  #: () -> String
  def to_s = "S#{v}"
end

class Bonus < Score
end

a = Score.new(1.5)
b = Score.new(0.5)
puts (a <=> b).inspect, [a, b].sort.map(&:to_s).inspect, [a, b, Bonus.new(0.1)].min.to_s
z = 0.0 #: Float
n = Score.new(z / z)
puts (n <=> a).inspect
begin
  puts [a, n].sort.inspect
rescue ArgumentError => e
  puts e.message
end
begin
  puts [Bonus.new(1.0), Bonus.new(z / z)].max.to_s
rescue ArgumentError => e
  puts e.message
end
x = 1.5 #: Float
puts (x <=> 2).inspect, (2 <=> x).inspect, ((x <=> 2.0) || 0) + 1
