# rbs_inline: enabled

class One
  #: (untyped) -> bool
  def ==(o) = o == 1
end

o = One.new
puts (1 == o).inspect, (1.0 == o).inspect, (2 == o).inspect, (o == 1).inspect
