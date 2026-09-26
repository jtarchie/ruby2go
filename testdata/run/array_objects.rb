# rbs_inline: enabled

class Array
  #: () -> E?
  def second = self[1]

  #: () -> Integer
  def twice_size = size * 2
end

class Shape
  #: () -> Float
  def area = 0.0

  #: () -> String
  def name = "shape"
end

class Circle < Shape
  #: (Float) -> void
  def initialize(r)
    @r = r
  end

  def area = 3.0 * @r * @r
  def name = "circle"
end

class Sq < Shape
  #: (Float) -> void
  def initialize(s)
    @s = s
  end

  def area = @s * @s
end

class Inbox
  attr_reader :items #: Array[String]

  #: () -> void
  def initialize
    @items = []
  end

  #: (String) -> self
  def add(s)
    @items << s
    self
  end
end

class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (untyped) -> bool
  def ==(other) = other.is_a?(Pt) && other.x == x

  #: () -> String
  def inspect = "P#{x}"
end

class Plain
  attr_accessor :n #: Integer

  #: (Integer) -> void
  def initialize(n)
    @n = n
  end
end

Box = Struct.new(:items) #: [Array[Integer]]

# an unannotated literal of sibling classes joins to their superclass
shapes = [Circle.new(1.0), Sq.new(2.0), Shape.new]
puts shapes.map(&:name).inspect, shapes.map(&:area).inspect, shapes.size
puts shapes.max_by(&:area)&.name.inspect, shapes.sort_by(&:area).map(&:name).inspect
puts shapes.select { |s| s.area > 1.0 }.map(&:name).inspect, shapes.min_by(&:area)&.name.inspect
two = [Circle.new(2.0), Sq.new(1.0)]
puts two.map(&:name).inspect, two.reverse.map(&:area).inspect
shapes << Sq.new(3.0)
puts shapes.last&.area.inspect, shapes.count

# an array ivar exposed by attr_reader is the same object
box = Inbox.new
box.add("a").add("c")
box.items << "b"
puts box.items.inspect, box.items.size, box.items.sort.inspect, box.items.equal?(box.items)
snapshot = box.items.dup
box.items.clear
puts snapshot.inspect, box.items.inspect

# user-defined == drives Array#==, delete; inspect uses the user's inspect
pts = [Pt.new(1), Pt.new(2)] #: Array[Pt]
puts ([Pt.new(2)] == [Pt.new(2)]), ([Pt.new(2)] == [Pt.new(3)])
puts [Pt.new(1), Pt.new(1)].uniq.size
puts pts.delete(Pt.new(1)).inspect, pts.inspect, pts.to_s, "#{pts}"
puts pts.delete(Pt.new(9)).inspect

# elements are references: mutating one through the array is seen outside
p1 = Plain.new(1)
plains = [p1, Plain.new(2)] #: Array[Plain]
plains.each { |pl| pl.n = pl.n + 10 }
puts p1.n, plains.map(&:n).inspect
plains << p1
puts plains.delete(p1)&.n.inspect, plains.size
plains.push(Plain.new(3))
plains.unshift(Plain.new(0))
puts plains.map(&:n).inspect

# arrays stored in a Hash and a Struct are shared, not copied
groups = {} #: Hash[String, Array[Integer]]
[1, 2, 3, 4, 5].each do |n|
  key = n.even? ? "even" : "odd"
  list = groups[key]
  if list
    list << n
  else
    groups[key] = [n]
  end
end
puts groups.inspect
evens = groups["even"]
evens << 100 if evens
puts groups.inspect, groups.map { |k, v| "#{k}:#{v.size}" }.inspect
bx = Box.new([1])
bx.items << 2
held = bx.items
held << 3
puts bx.items.inspect, bx.inspect
puts [1, 2, 3].second.inspect, ["a"].second.inspect, [].twice_size, shapes.twice_size, shapes.second&.name.inspect
