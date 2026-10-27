module Lib
  def self.first_byte(s)
    s.getbyte(0) + 1
  end

  def self.sum_to(n)
    total = 0
    n.times { |i| total += i }
    total
  end

  def self.doubled(xs)
    xs.map { |x| x * 2 }
  end

  def self.show(x)
    "<#{x}>"
  end
end
