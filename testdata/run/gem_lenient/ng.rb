class Ng
  def initialize
    @names = Hash.new
    @names["a"] = 1
    @args = Array.new
    @args << "x"
  end

  # a reset to nil: a gem's container ivar is optional after it (getoptlong's terminate)
  def reset
    @names = nil
    @args = nil
    self
  end

  def names = @names

  def args = @args

  # a nil in a multiple assignment types the local by its later writes (getoptlong's get)
  def multi
    name, arg = nil, ""
    name = "n" if @names
    [name, arg]
  end

  # nil stored into a fresh Array's element widens it to Array[T?] (prime's sieve, decision 411)
  def sieve(max)
    segment = (2..max).to_a
    segment.each_index { |i| segment[i] = nil if segment[i].even? && segment[i] > 2 }
    segment.compact!
    segment
  end

  # an endless Float range runs until break (prime's `(1..Float::INFINITY).each`)
  def upto_inf(n)
    seen = []
    (1..Float::INFINITY).each do |i|
      break if i > n

      seen << i
    end
    seen
  end
end
