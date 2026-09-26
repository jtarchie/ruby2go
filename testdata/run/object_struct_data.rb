# rbs_inline: enabled

# Struct.new and Data.define: both annotation forms, accessors, new (positional, keyword, optional members), ==, to_a, to_h, members, inspect, with.

Point = Struct.new(
  :x, #: Integer
  :y  #: Integer
)

Pair = Struct.new(:left, :right) #: [String, Float]

Rec = Struct.new(:id, :name, :note) do
  #: () -> String
  def label = "#{id}:#{name || "-"}:#{note || "-"}"

  #: (Integer) -> Rec
  def self.numbered(n) = new(n)
end #: [Integer, String?, String?]

class Named < Rec
  def label = "named " + super
end

module Geo
  Coord = Data.define(
    :lat, #: Float
    :lng  #: Float
  ) do
    #: () -> String
    def to_s = "(#{lat}, #{lng})"

    #: () -> Float
    def sum = lat + lng
  end

  Tag = Struct.new(:text) #: [String]
end

Money = Data.define(:cents, :currency) #: [Integer, String]

origin = Point.new(0, 0)
pt = Point.new(3, -4)
puts pt.x.inspect, pt.y.inspect, pt.inspect, pt.to_s, origin.inspect
pt.x = 10
pt.y = pt.y - 1
puts pt.inspect, pt.to_a.inspect, pt.to_h.inspect, pt.members.inspect, Point.members.inspect
puts (pt == Point.new(10, -5)).inspect, (pt == origin).inspect, (pt != origin).inspect, (pt == pt).inspect
puts Point.new(y: 2, x: 1).inspect, Point.new(x: 7, y: 8).to_a.inspect
puts Point.name, Point.new(1, 1).class, Point.inspect

pair = Pair.new("ünï", 1.0)
puts pair.inspect, pair.left, pair.right.inspect, Pair.new("", -0.5).inspect
pair.left = "quote\"d"
puts pair.inspect, pair.to_h.inspect

puts Rec.new(1).inspect, Rec.new(2, "two").inspect, Rec.new(3, "three", "n").inspect
puts Rec.new(1).label, Rec.new(2, "b").label, Rec.numbered(9).label, Named.new(4, "d").label
puts Named.new(5).inspect, Named.numbered(6).inspect, Rec.new(id: 7, note: "kw").inspect
puts (Named.new(1) == Named.new(1)).inspect, (Rec.new(1, "a") == Rec.new(1, "b")).inspect
o = Rec.new(8)
o.name = "late"
puts o.label, o.to_a.inspect, o.members.inspect

here = Geo::Coord.new(lat: 1.5, lng: -2.0)
there = here.with(lng: 3.25)
same = here.with
puts here.inspect, there.inspect, here, there, here.sum.inspect, there.sum.inspect
puts same.inspect, (same == here).inspect, (here == there).inspect, (here == Geo::Coord.new(1.5, -2.0)).inspect
puts here.to_h.inspect, Geo::Coord.members.inspect, here.members.inspect, Geo::Coord.name
puts here.with(lat: 0.0, lng: 0.0).inspect, Geo::Tag.new("t").inspect, Geo::Tag.new("t").to_a.inspect

m = Money.new(1999, "EUR")
puts m.inspect, m.cents.inspect, Money.new(currency: "USD", cents: -5).inspect, m.with(cents: 0).inspect
puts (m == Money.new(1999, "EUR")).inspect, (m == m.with(currency: "GBP")).inspect, m.to_h.inspect
puts Money.new(9_000_000_000_000, "JPY").cents.inspect

points = [Point.new(2, 1), Point.new(1, 2), Point.new(1, 1)] #: Array[Point]
puts points.sort_by { |q| (q.x * 10) + q.y }.map(&:to_a).inspect
puts points.map(&:x).inspect, points.select { |q| q.y == 1 }.inspect

Inner = Struct.new(:a) #: [Integer]
Wrap = Struct.new(:inner, :list, :map) #: [Inner, Array[String], Hash[Symbol, Integer]]
Kw = Struct.new(:type, :range, :func, :map, :go, :select) #: [String, Integer, String, String, bool, Integer]

class Thing
  attr_reader :type #: String
  attr_accessor :interface #: Integer

  #: () -> void
  def initialize
    @type = "t"
    @interface = 1
  end

  #: () -> String
  def len = "len"

  #: () -> String
  def string = "string"

  #: () -> String
  def default = "default"
end

w = Wrap.new(Inner.new(1), ["a", "b"], { k: 1 })
puts w.inspect, w.to_a.inspect, w.to_h.inspect
k = Kw.new("x", 2, "f", "m", true, 0)
puts k.inspect, k.type, k.range.inspect, k.go.inspect
k.type = "y"
puts k.to_h.inspect
t = Thing.new
t.interface = 5
puts t.type, t.interface.inspect, t.len, t.string, t.default
Val = Data.define(:v) #: [Float]
puts Val.new(-0.0).inspect, Val.new(1e20).inspect, Val.new(1.0 / 3).inspect

# Self-referential member types (an optional link, an array of children); a Struct subclass
# adding its own attr and initialize that calls super.
ListNode = Struct.new(:val, :nxt) #: [Integer, ListNode?]
Tree = Struct.new(:val, :kids) #: [Integer, Array[Tree]]

#: (ListNode?) -> Integer
def total(n)
  sum = 0
  while n
    sum += n.val
    n = n.nxt
  end
  sum
end

#: (Tree) -> Integer
def tsum(t) = t.val + t.kids.reduce(0) { |a, k| a + tsum(k) }

list = ListNode.new(1, ListNode.new(2, ListNode.new(3)))
puts total(list).inspect, list.nxt.inspect, ListNode.new(9).inspect, total(nil).inspect
list.nxt = nil
puts total(list).inspect, (list == ListNode.new(1)).inspect
tr = Tree.new(1, [Tree.new(2, []), Tree.new(3, [Tree.new(4, [])])])
puts tsum(tr).inspect, tr.kids.size.inspect, tr.kids.fetch(0).inspect

class Pt3 < Point
  attr_reader :z #: Integer

  #: (Integer, Integer, Integer) -> void
  def initialize(x, y, z)
    super(x, y)
    @z = z
  end

  #: () -> Integer
  def sum = x + y + z
end

p3 = Pt3.new(1, 2, 3)
puts p3.sum.inspect, p3.x.inspect, p3.to_a.inspect, p3.inspect, p3.members.inspect, p3.z.inspect
p3.x = 10
puts p3.sum.inspect, p3.to_h.inspect, Pt3.members.inspect, Pt3.name

# Struct instances are references: a second name sees the mutation; equal? is identity, == is by value.
pa = Point.new(1, 2)
pb = pa
pb.x = 9
puts pa.x.inspect, pa.equal?(pb).inspect, pa.equal?(Point.new(9, 2)).inspect, (pa == Point.new(9, 2)).inspect
puts (pa == nil).inspect, (pa == 9).inspect, (pa == [9, 2]).inspect, (Money.new(1, "x") == Point.new(1, 2)).inspect, (pa != nil).inspect
