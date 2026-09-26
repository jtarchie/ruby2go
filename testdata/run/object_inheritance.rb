# rbs_inline: enabled

# Inheritance, virtual dispatch on self, overriding, super in all its forms.

class Base
  attr_reader :tag #: String

  #: (String) -> void
  def initialize(tag)
    @tag = tag
  end

  #: () -> String
  def name = "Base"

  #: () -> Integer
  def weight = 1

  #: () -> String
  def describe = "#{name}[#{tag}] w=#{weight}"

  #: (Integer) -> Integer
  def scale(n) = n * weight

  #: (String, ?String) -> String
  def greet(who, punct = "!") = "hi #{who}#{punct}"

  #: () -> String
  def to_s = "<#{name} #{tag}>"
end

class Middle < Base
  def name = "Middle"

  def weight = super * 10

  def scale(n) = super(n) + 1

  def greet(who, punct = "?") = super.upcase
end

class Leaf < Middle
  #: (String, Integer) -> void
  def initialize(tag, extra)
    super(tag)
    @extra = extra
  end

  def name = "Leaf#{@extra}"

  def weight = super + @extra

  def describe = "leaf: #{super}"

  #: () -> Integer
  def extra = @extra
end

class Empty < Base
end

class Fixed < Base
  #: () -> void
  def initialize = super("fixed")
end

class Root
  #: () -> void
  def initialize
    super
    @n = 7 #: Integer
  end

  #: () -> Integer
  def n = @n
end

class Summer
  SCALE = 2

  #: (*Integer) -> void
  def initialize(*nums)
    @nums = nums
  end

  #: () -> Integer
  def total = @nums.reduce(0) { |a, b| a + b }

  #: () -> Integer
  def scaled = total * SCALE
end

class KidSummer < Summer
  SCALE = 5

  def initialize(*nums) = super

  #: () -> Integer
  def kid_scaled = total * SCALE
end

b = Base.new("b")
m = Middle.new("m")
l = Leaf.new("l", 3)
e = Empty.new("e")
f = Fixed.new

puts b.describe, m.describe, l.describe, e.describe, f.describe
puts b.weight.inspect, m.weight.inspect, l.weight.inspect, e.weight.inspect
puts b.scale(2).inspect, m.scale(2).inspect, l.scale(2).inspect, b.scale(0).inspect, m.scale(-4).inspect
puts b.greet("a").inspect, m.greet("a").inspect, m.greet("a", ".").inspect, l.greet("z", "").inspect
puts b, m, l, e, f
puts b.to_s.inspect, l.to_s.inspect
puts Root.new.n.inspect

all = [b, m, l, e, f] #: Array[Base]
puts all.map(&:name).inspect
puts (all.map { |x| x.weight }).inspect
puts all.map(&:tag).inspect
puts (all.reduce(0) { |acc, x| acc + x.scale(1) }).inspect

puts l.is_a?(Base).inspect, l.is_a?(Middle).inspect, l.is_a?(Leaf).inspect, m.is_a?(Leaf).inspect, b.kind_of?(Middle).inspect
puts (all.map { |x| x.is_a?(Middle) }).inspect
puts l.is_a?(Object).inspect, l.is_a?(BasicObject).inspect, l.is_a?(Kernel).inspect, l.is_a?(String).inspect

puts (b == b).inspect, (b == Base.new("b")).inspect, (b != Base.new("b")).inspect, (b != b).inspect
puts b.equal?(b).inspect, b.equal?(m).inspect, (!b).inspect, b.nil?.inspect

puts Summer.new(1, 2).total.inspect, KidSummer.new(1, 2, 3).total.inspect, KidSummer.new.total.inspect
puts Summer.new(1).scaled.inspect, KidSummer.new(1).scaled.inspect, KidSummer.new(1, -3).kid_scaled.inspect

all.each do |x|
  if x.is_a?(Leaf)
    puts "leaf extra #{x.extra}"
  elsif x.is_a?(Middle)
    puts "middle #{x.tag}"
  end
end
found = all.find { |x| x.is_a?(Fixed) }
puts found.tag if found

class Animal
  #: () -> String
  def name = "animal"
end

class Dog < Animal
  #: () -> String
  def bark = "woof"
end

class Puppy < Dog
end

class Cat < Animal
end

#: (Animal) -> String
def kind(a)
  case a
  when Puppy then "puppy #{a.bark}"
  when Dog then "dog #{a.bark}"
  when Cat then "cat"
  else "other"
  end
end

list = [Animal.new, Dog.new, Puppy.new, Cat.new] #: Array[Animal]
list.each { |a| puts kind(a) }

# zsuper passes the parameters' current values; super() passes none; super inside a block.
class Calc
  #: (Integer, ?Integer) -> Integer
  def compute(n, k = 10) = n * k

  #: () -> String
  def plain = "calc"

  #: () -> Array[String]
  def parts = ["a", "b"]

  #: (String) -> String
  def tag(s) = "<#{s}>"
end

class KidCalc < Calc
  def compute(n, k = 3)
    n += 1
    k *= 2
    super
  end

  def plain = super() + "!"

  def parts = super.map { |p| tag(p) + plain }

  def tag(s) = [1, 2].map { |i| super(s * i) }.join
end

class GrandCalc < KidCalc
  def compute(n, k = 1) = super(n) + super(n, k)
end

kc = KidCalc.new
puts kc.compute(1).inspect, kc.compute(1, 1).inspect, kc.compute(-1, 0).inspect, kc.plain, kc.parts.inspect, kc.tag("x")
puts GrandCalc.new.compute(1).inspect, GrandCalc.new.compute(2, 5).inspect, Calc.new.compute(2).inspect

# Three-level initialize chain with defaults; sibling subclasses with same-named ivars of different types.
class Node
  attr_reader :id #: Integer

  #: (?Integer) -> void
  def initialize(id = 1)
    @id = id
  end
end

class Named < Node
  attr_reader :label #: String

  #: (String, ?Integer) -> void
  def initialize(label, id = 2)
    super(id)
    @label = label
  end
end

class Leafy < Named
  #: () -> void
  def initialize
    super("leafy")
  end
end

class IntBox < Node
  #: () -> void
  def initialize
    super()
    @v = 41
  end

  #: () -> Integer
  def v = @v + 1
end

class StrBox < Node
  #: () -> void
  def initialize
    super(9)
    @v = "s"
  end

  #: () -> String
  def v = @v * 2
end

puts Node.new.id.inspect, Named.new("n").id.inspect, Named.new("m", 7).label, Leafy.new.id.inspect, Leafy.new.label
puts IntBox.new.v.inspect, StrBox.new.v, StrBox.new.id.inspect, IntBox.new.id.inspect
nodes = [Node.new, Leafy.new, IntBox.new, StrBox.new] #: Array[Node]
puts nodes.map(&:id).inspect, nodes.map { |x| x.class.name }.inspect, nodes.select { |x| x.is_a?(Named) }.size.inspect

# Objects as Hash keys and in collections use identity unless == is overridden.
k1 = nodes.fetch(0)
k2 = nodes.fetch(1)
by_obj = { k1 => "one" } #: Hash[Node, String]
by_obj[k2] = "two"
puts by_obj[k1].inspect, by_obj[k2].inspect, by_obj[Node.new].inspect, by_obj.size.inspect
puts nodes.include?(k1).inspect, [k1].include?(k2).inspect, [k1, k1, k2].uniq.size.inspect, (k1 == k2).inspect
puts nodes.max_by(&:id).class, nodes.min_by(&:id).id.inspect, nodes.sort_by { |x| -x.id }.map(&:id).inspect
puts nodes.group_by(&:class).keys.inspect, k1.equal?(nodes.fetch(0)).inspect

# The abstract-method idiom: raise NotImplementedError in the base, which a bare rescue does not catch.
class Figure
  #: () -> Integer
  def area = raise(NotImplementedError, "#{self.class.name}#area")

  #: () -> String
  def show = "area #{area}"
end

class Sq < Figure
  def area = 4
end

puts Sq.new.show
begin
  Figure.new.show
rescue NotImplementedError => e
  puts "#{e.class}: #{e.message}"
end
begin
  begin
    Figure.new.area
  rescue => e
    puts "bare rescue caught #{e.class}"
  end
rescue NotImplementedError
  puts "only NotImplementedError caught it"
end

# An inherited `-> self` method keeps the subclass type through a chain, a local, and an array.
class Builder
  #: () -> void
  def initialize
    @parts = [] #: Array[String]
  end

  #: (String) -> self
  def add(part)
    @parts << part
    self
  end

  #: () -> String
  def build = @parts.join("-")
end

class HtmlBuilder < Builder
  #: () -> String
  def html = "<p>#{build}</p>"
end

hb = HtmlBuilder.new
puts hb.add("a").add("b").html, hb.add("c").build
hx = hb.add("d")
puts hx.html, hx.equal?(hb).inspect
builders = [Builder.new, HtmlBuilder.new] #: Array[Builder]
puts builders.map { |x| x.add("z").build }.inspect
