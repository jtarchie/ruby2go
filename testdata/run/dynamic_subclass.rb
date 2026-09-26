# rbs_inline: enabled

class Shape
  attr_reader :tag #: String

  #: (String) -> void
  def initialize(tag)
    @tag = tag
  end

  #: () -> String
  def describe = "#{tag}: area #{area}"

  #: () -> String
  def self.kind = "shape"
end

class Sq < Shape
  #: () -> Integer
  def area = 4

  #: (Integer) -> Integer
  def scaled(k) = area * k

  #: () -> String
  def self.corners = "four"
end

class Tri < Shape
  #: () -> Float
  def area = 1.5

  #: (Integer, ?Integer) -> Float
  def scaled(k, extra = 0) = area * k.to_f + extra.to_f

  #: () -> String
  def self.corners = "three"
end

class Blob < Shape
end

#: (Shape) -> String
def area_of(s) = s.area.inspect

#: (Shape) -> String
def sent_area(s) = s.send(:area).inspect

#: (Integer) -> Shape?
def pick(n) = n.zero? ? nil : (n.positive? ? Sq.new("p") : Blob.new("q"))

puts "-- a method only subclasses define dispatches at run time"
puts Sq.new("sq").describe, Tri.new("tri").describe
puts area_of(Sq.new("a")), area_of(Tri.new("b")), sent_area(Sq.new("c")), sent_area(Tri.new("d"))
shapes = [Sq.new("s"), Tri.new("t"), Blob.new("b")] #: Array[Shape]
shapes.each do |s|
  puts s.respond_to?(:area), s.respond_to?(:scaled), s.respond_to?(:tag)
  begin
    puts s.scaled(2).inspect
    puts s.scaled(2, 1).inspect
  rescue ArgumentError => e
    puts "ArgumentError: #{e.message}"
  rescue NoMethodError => e
    puts "NoMethodError: #{e.message}"
  end
end

puts "-- a base instance raises NoMethodError, or NameError for a bare name"
begin
  area_of(Shape.new("plain"))
rescue NoMethodError => e
  puts "NoMethodError: #{e.message}"
end
begin
  Blob.new("blob").describe
rescue NameError => e
  puts "#{e.class}: #{e.message}"
end

puts "-- class objects typed singleton(Base) and Module"
classes = [Sq, Tri, Blob] #: Array[singleton(Shape)]
classes.each { |k| puts k.new("x").tag, k.name, k.kind, k.to_s, k.inspect }
mods = [Sq, Tri, Shape] #: Array[Module]
mods.each do |m|
  puts m.name
  puts m.corners
rescue NoMethodError => e
  puts "NoMethodError: #{e.message}"
end

puts "-- subclass-only methods through singleton(Base) and Base?"
[Sq, Blob].each do |k|
  kk = k #: singleton(Shape)
  puts kk.respond_to?(:corners), kk.respond_to?(:kind)
  begin
    puts kk.corners
  rescue NoMethodError => e
    puts e.message
  end
end
puts pick(1)&.area.inspect, pick(0)&.area.inspect, pick(1).respond_to?(:area), pick(-1).respond_to?(:area)
begin
  pick(-1)&.area
rescue NoMethodError => e
  puts e.message
end
