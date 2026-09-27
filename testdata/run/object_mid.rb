# rbs_inline: enabled

# an attr writer call's value is the assigned value, not the setter's return
class Box
  attr_accessor :n #: Integer

  #: () -> void
  def initialize
    @n = 0
  end
end

box_a = Box.new
ret_a = (box_a.n = 42)
puts ret_a.inspect, box_a.n.inspect
box_c = Box.new
ret_x = box_c.n = 7
puts ret_x.inspect

# a class body can call its own class methods and new while defining constants
class Temp
  attr_reader :deg #: Integer

  #: (Integer) -> void
  def initialize(deg)
    @deg = deg
  end

  #: (Integer) -> Temp
  def self.of(d) = new(d)

  FREEZING = of(0)
  BOILING = new(100)
end

puts Temp::FREEZING.deg.inspect, Temp::BOILING.deg.inspect

# yielding class methods on a module and a class iterate from a call site
module Colors
  #: () { (String) -> void } -> void
  def self.each
    yield "red"
    yield "green"
  end
end

class Counter
  #: (Integer) { (Integer) -> void } -> void
  def self.upto(n)
    i = 0
    while i < n
      yield i
      i += 1
    end
  end
end

Colors.each { |color| puts color }
Counter.upto(3) { |i| puts i }

# inherit=false hides a superclass constant from constants/const_defined?/const_get
class Parent
  COLOR = "red"
end

class Child < Parent
  SIZE = 5
end

puts Child.constants(false).inspect
puts Child.const_defined?(:COLOR, false).inspect
begin
  cval = Child.const_get(:COLOR, false)
  puts cval
rescue NameError => name_err
  puts "NameError: #{name_err.message}"
end

# Child::X resolves constants and nested classes through the superclass
class PathParent
  COLOR = "red"

  class Nested
    #: () -> String
    def hi = "nested"
  end
end

class PathChild < PathParent
  SIZE = 5
end

puts PathChild::SIZE.inspect
puts PathChild::COLOR
puts PathChild::Nested.new.hi

# a struct-class argument to generic then/reduce/include?, and singleton arrays
class Vec
  attr_reader :v #: Integer

  #: (Integer) -> void
  def initialize(v)
    @v = v
  end

  #: (Vec) -> Vec
  def plus(o) = Vec.new(v + o.v)
end

class Vec3 < Vec
end

gvs = [Vec.new(1), Vec.new(-2), Vec.new(5)] #: Array[Vec]
puts Vec.new(4).then { |vb| vb.v * 2 }.inspect
puts Vec.new(1).then { |vb| "v=#{vb.v}" }
puts gvs.reduce(Vec.new(0)) { |acc, vb| acc.plus(vb) }.v.inspect
puts gvs.include?(Vec.new(5)).inspect
klasses = [Vec, Vec3] #: Array[singleton(Vec)]
puts klasses.include?(Vec3).inspect

# user-defined self.name/to_s/inspect are inherited by subclasses
class Widget
  #: () -> String
  def self.name = "CustomWidget"
end

class SubWidget < Widget
end

class Gadget
  #: () -> String
  def self.to_s = "GadgetClass"

  #: () -> String
  def self.inspect = "GadgetInspect"
end

class SubGadget < Gadget
end

puts Widget.name, SubWidget.name, SubWidget.new.class.name
puts Gadget, SubGadget, "#{SubGadget}", SubGadget.inspect, [SubGadget].inspect

# is_a?(Module) is false for a class that neither includes it nor has subclasses
module Walker
end

class Doc
end

class Page
  include Walker
end

d = Doc.new
pg = Page.new
puts d.class, pg.class
puts d.is_a?(Walker).inspect
puts pg.is_a?(Walker).inspect

# an ivar assignment as a method's last expression is its return value
class Memo
  # @rbs @last: Integer?

  #: () -> void
  def initialize
    @last = nil
  end

  #: (Integer) -> Integer
  def store(n)
    @last = n * 2
  end

  #: () -> Integer?
  def last = @last
end

m = Memo.new
puts m.last.inspect
puts m.store(21).inspect
puts m.last.inspect

# `@x ||= [] #: T` types the ivar from the trailing annotation
class ItemBox
  #: () -> Array[String]
  def items
    @items ||= [] #: Array[String]
  end
end

class Reg
  #: () -> Hash[String, Integer]
  def self.table
    @table ||= {} #: Hash[String, Integer]
  end
end

ibox = ItemBox.new
ibox.items << "a"
Reg.table["x"] = 1
puts ibox.items.inspect, Reg.table.inspect

# a local declared as the superclass can be reassigned a subclass instance or class
class Base
  #: () -> String
  def name = "Base"
end

class Leaf < Base
  def name = "Leaf"
end

sub_a = Base.new #: Base
sub_a = Leaf.new
puts sub_a.name

sub_x = Base.new
sub_x = Leaf.new
puts sub_x.name

sub_y = Leaf.new #: Base
puts sub_y.name
sub_y = Base.new
puts sub_y.name

sub_k = Leaf #: singleton(Base)
puts sub_k.name
sub_k = Base
puts sub_k.name

# a method named like its class (calc/Calc, node/Node) survives subclassing
class Calc
  #: (Integer) -> Integer
  def calc(n) = n * 2
end

class KidCalc < Calc
  def calc(n) = super + 1
end

class Node
  attr_reader :node #: String

  #: () -> void
  def initialize
    @node = "node"
  end
end

class NodeLeaf < Node
end

puts Calc.new.calc(1).inspect, KidCalc.new.calc(2).inspect, NodeLeaf.new.node

# an included module's constant resolves unqualified in the class and its subclasses
module Config
  LIMIT = 5
end

class Uses
  include Config

  #: () -> Integer
  def lim = LIMIT
end

class Sub < Uses
  #: () -> Integer
  def twice = LIMIT * 2
end

puts Uses.new.lim.inspect, Sub.new.twice.inspect
puts Uses::LIMIT.inspect, Sub::LIMIT.inspect
