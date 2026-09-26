# skip: decision 3's names are not injective against camel-cased method names (+/plus -> Plus, []/idx -> Idx, -@/neg -> Neg, / and Ruby's own div -> Div, ** and pow -> Pow, empty?/empty_q -> EmptyQ), so go build fails (redeclared)
# rbs_inline: enabled

class Meter
  #: (Integer) -> Integer
  def +(o) = o + 1
  #: (Integer) -> Integer
  def plus(o) = o + 2
  #: (Integer) -> Integer
  def [](i) = i * 10
  #: (Integer) -> Integer
  def idx(i) = i * 100
  #: () -> Integer
  def -@ = -1
  #: () -> Integer
  def neg = -2
  #: (Integer) -> Integer
  def /(o) = 100 / o
  #: (Integer) -> Integer
  def div(o) = 1000 / o
  #: (Integer) -> Integer
  def **(o) = o * o
  #: (Integer) -> Integer
  def pow(o) = o * o * o
  #: () -> bool
  def empty? = true
  #: () -> bool
  def empty_q = false
end

m = Meter.new
puts (m + 1).inspect, m.plus(1).inspect, m[1].inspect, m.idx(1).inspect
puts (-m).inspect, m.neg.inspect, (m / 10).inspect, m.div(10).inspect, (m ** 3).inspect, m.pow(3).inspect
puts m.empty?.inspect, m.empty_q.inspect
