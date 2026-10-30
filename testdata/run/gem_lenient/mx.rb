# Matrix-shaped gem code (decision 414).
class Mx
  def initialize(rows)
    @rows = rows
    @d = Array.new(rows.size, 0)
    @v = Array.new(rows.size) { Array.new(rows.size, 0) }
  end

  def conv(b) = b.to_a

  def solve(b)
    if b.is_a? Array
      m = [b.size]
      m
    else
      b = conv(b)
      m = b.dup
      m
    end
  end

  def fill
    @d[0] = Math.sqrt(2.0)
    @d[1] = @rows[0][0] / 3.0
    @v[0][1] = 0.5
    @v[1][0] = Rational(1, 3)
    [@d, @v]
  end

  def each_with_index
    return to_enum :each_with_index unless block_given?

    @rows.each_with_index { |row, i| row.each_with_index { |e, j| yield e, i, j } }
    self
  end

  def columns(imag)
    imag.each_with_index.map do |x, i|
      if x.zero?
        Array.new(@rows.size) { |j| @rows[j][i] }
      else
        Array.new(@rows.size) { |j| Complex(@rows[j][i], x) }
      end
    end
  end

  def collect(which = :all, &block)
    return to_enum(:collect, which) unless block_given?

    out = []
    each_with_index { |e, i, j| out << block.call(e) if which == :all || i == j }
    out
  end
  alias_method :map, :collect

  def index(*args)
    which = args.size == 2 ? args.pop : :all
    return to_enum :find_index, which, *args unless block_given? || args.size == 1

    each_with_index { |e, i, j| return i, j if args.size == 1 ? e == args.first : yield(e) }
    nil
  end
  alias_method :find_index, :index

  def strict_upper
    each_with_index.select { |_, i, j| j > i }.map(&:first).each
  end

  def strict_upper_even? = strict_upper.all?(&:even?)

  def col(j)
    return [j, :plain] unless block_given?

    yield j
  end

  def dyn_col = [self, 1, "a", :b][0].col(1)

  def self.diagonal(*values)
    Array.new(values.size) do |j|
      row = Array.new(values.size, 0)
      row[j] = values[j]
      row
    end
  end

  def self.ramp(n)
    row = Array.new(n, 0)
    n.times { |i| row[i] = Rational(i, 2) if i.odd? }
    row
  end

  def self.build(r, c = r)
    Array.new(r) { |i| Array.new(c) { |j| yield i, j } }
  end
end

class Mx
  def twin = MxPair.new(@rows, @rows.reverse)

  def twin_solve
    a, b = twin
    [a.solve(1..2), b.solve(3..4), Mx2.new([]).solve([])]
  end
end

# A subclass: Mx objects are a Go interface, which `a, b = twin` must not wrap (decision 414).
class Mx2 < Mx
end

class MxPair
  def initialize(a, b)
    @a = a
    @b = b
  end

  def to_ary = [Mx.new(@a), Mx.new(@b)]
end

# EigenvalueDecomposition's shape (decision 420): rows seeded with Integer 0s, then written with Floats through the
# untyped @v; and a symmetric path whose @v is another value, so @v is untyped.
class MxEigen
  def initialize(size, symmetric)
    @v = symmetric ? [[1.5]] : Array.new(size) { Array.new(size, 0) }
  end

  def scale(f)
    @v.each_index { |i| @v[i].each_index { |j| @v[i][j] = (i + 1) * f + j } }
    @v
  end
end
