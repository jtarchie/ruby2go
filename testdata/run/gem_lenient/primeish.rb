# prime.rb's shapes (decision 417): an iterator's value with a valueless break in its block.
class Primeish
  def witness?(r, x, m)
    return false if r.times do
      x = (x * x) % 97
      break if x == m
    end

    true
  end

  def scan(xs, stop)
    xs.each do |x|
      break if x == stop
    end
  end
end

# prime's PseudoPrimeGenerator (decision 418): `return self.dup unless block_given?`, then rewind on the copy.
class PrimeishGen
  include Enumerable

  def initialize
    @i = -1
  end

  def succ
    @i += 1
    [2, 3, 5, 7, 11][@i]
  end
  alias next succ

  def rewind
    initialize
  end

  def each
    return dup unless block_given?

    loop do
      v = succ
      break if v.nil?

      yield v
    end
  end
end

class PrimeishFront
  def each(generator = PrimeishGen.new, &block)
    generator.each(&block)
  end

  #: (untyped) ?{ (untyped) -> untyped } -> untyped
  def each_any(generator, &block)
    generator.each(&block)
  end
end

class PrimeishOddGen < Struct.new(:n)
  def each
    return [n].each unless block_given?

    yield n
  end
end

# prime's each with an upper bound: its value is the last block value, through `break last_value` (decision 418).
class PrimeishLast
  include Enumerable

  def initialize(ubound) = @ubound = ubound

  def each
    return dup unless block_given?

    last_value = nil
    i = 0
    loop do
      i += 1
      break last_value if i > @ubound

      last_value = yield i
    end
  end
end

class PrimeishBound
  def each(ubound, generator = PrimeishLast.new(ubound), &block)
    generator.each(&block)
  end
end
