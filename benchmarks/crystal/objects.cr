# Crystal port of benchmarks/objects.rb, for the README comparison column.
# Vec is a struct (value type), as a Crystal programmer would write it. Float sums are plain
# (Ruby's Array#sum is Kahan-Babuska); the rounded output is identical.

struct Vec
  getter x : Float64
  getter y : Float64

  def initialize(@x, @y)
  end

  def +(other : Vec) : Vec
    Vec.new(@x + other.x, @y + other.y)
  end

  def -(other : Vec) : Vec
    Vec.new(@x - other.x, @y - other.y)
  end

  def *(k : Float64) : Vec
    Vec.new(@x * k, @y * k)
  end

  def length : Float64
    Math.sqrt(@x * @x + @y * @y)
  end
end

class Body
  property pos : Vec
  property vel : Vec
  getter mass : Float64

  def initialize(@pos, @vel, @mass)
  end
end

abstract class Shape
  include Comparable(Shape)

  def area : Float64
    0.0
  end

  def <=>(other : Shape)
    (area <=> other.area) || 0
  end
end

class Circle < Shape
  def initialize(@r : Float64)
  end

  def area : Float64
    3.14159 * @r * @r
  end
end

class Rect < Shape
  def initialize(@w : Float64, @h : Float64)
  end

  def area : Float64
    @w * @h
  end
end

class Square < Rect
  def initialize(s : Float64)
    super(s, s)
  end
end

bodies = (0...40).map { |i| Body.new(Vec.new(i.to_f, (i * 3 % 7).to_f), Vec.new(0.0, 0.0), 1.0 + (i % 3)) }
60.times do
  bodies.each do |a|
    acc = Vec.new(0.0, 0.0)
    bodies.each do |b|
      next if a.same?(b)

      d = b.pos - a.pos
      dist = d.length + 0.01
      acc += d * (b.mass / (dist * dist * dist))
    end
    a.vel += acc * 0.001
  end
  bodies.each { |b| b.pos += b.vel }
end
puts bodies.sum { |b| b.pos.length }.round(6)

shapes = [] of Shape
200_000.times do |i|
  shapes << case i % 3
  when 0 then Circle.new((i % 10).to_f)
  when 1 then Rect.new((i % 7).to_f, (i % 5).to_f)
  else        Square.new((i % 4).to_f)
  end
end
puts shapes.sum(&.area).round(2)
puts shapes.max?.try(&.area)
puts shapes.count { |s| s > shapes[1] }
