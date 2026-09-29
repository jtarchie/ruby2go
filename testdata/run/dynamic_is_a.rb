# rbs_inline: enabled

module Named
  #: () -> String
  def label = "named"
end

class Vehicle
  #: () -> String
  def kind = "vehicle"

  #: () -> bool
  def car? = is_a?(Car)

  #: () -> bool
  def self_truck? = self.kind_of?(Truck)
end

class Car < Vehicle
  include Named

  #: () -> String
  def kind = "car"
end

class Truck < Vehicle
end

#: (untyped) -> untyped
def ident(v) = v

#: (Vehicle) -> String
def checks(v)
  [v.is_a?(Vehicle), v.is_a?(Car), v.kind_of?(Truck), v.is_a?(Object), v.is_a?(BasicObject), v.is_a?(Kernel)].inspect
end

#: (untyped) -> String
def untyped_checks(v)
  [v.is_a?(Integer), v.is_a?(Float), v.is_a?(String), v.is_a?(Symbol), v.is_a?(Array), v.is_a?(Hash), v.is_a?(Vehicle), v.is_a?(Car), v.is_a?(Object)].inspect
end

module Geo
  class Pt
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end
  end

  class Pt3 < Pt
  end
end

#: (untyped) -> String
def walk(v)
  if v.is_a?(Array)
    v.map { |e| e.to_s }.join("+")
  elsif v.is_a?(Hash)
    out = [] #: Array[String]
    v.each { |k, x| out << "#{k}=#{x}" }
    out.join(",")
  elsif v.is_a?(Geo::Pt)
    "pt #{v.x}"
  else
    "leaf"
  end
end

puts "-- static types decide is_a? on primitives"
puts [1.is_a?(Integer), 1.is_a?(Float), 1.is_a?(Comparable), 1.is_a?(Object), 1.kind_of?(BasicObject), 1.is_a?(Kernel)].inspect
puts [1.5.is_a?(Float), 1.5.is_a?(Integer), 1.5.is_a?(Comparable)].inspect
puts ["s".is_a?(String), "s".is_a?(Comparable), "s".is_a?(Symbol), "".kind_of?(Object)].inspect
sym = :s
arr = [1]
hsh = { "a" => 1 }
puts [sym.is_a?(Symbol), sym.is_a?(String), sym.is_a?(Comparable)].inspect, sym.inspect
puts [arr.is_a?(Array), arr.is_a?(Enumerable), arr.is_a?(Hash), arr.kind_of?(Object)].inspect, arr.inspect
puts [hsh.is_a?(Hash), hsh.is_a?(Enumerable), hsh.is_a?(Array)].inspect, hsh.inspect
puts [true.is_a?(Object), false.kind_of?(Object), nil.is_a?(Integer)].inspect
pair = [1, "a"]
puts [pair.is_a?(Array), pair.is_a?(Object), pair.is_a?(Hash)].inspect, pair.inspect

puts "-- struct classes: constant when the static type decides, else a run-time check"
puts checks(Vehicle.new), checks(Car.new), checks(Truck.new)
car = Car.new
truck = Truck.new
puts [car.is_a?(Vehicle), car.is_a?(Truck), car.is_a?(Named), truck.kind_of?(Car)].inspect, car.kind, truck.kind

puts "-- untyped values: a type assertion"
[1, -2.5, "str", :sym, [1], { 1 => 2 }, Vehicle.new, Car.new, Truck.new, nil, true, false].each { |v| puts untyped_checks(v) }
puts ident(Car.new).is_a?(Vehicle), ident(Truck.new).is_a?(Car), ident([]).kind_of?(Array), ident({}).kind_of?(Hash)

puts "-- exceptions"
errs = [ArgumentError.new("a"), KeyError.new("k"), RuntimeError.new("r"), StandardError.new("s")] #: Array[StandardError]
errs.each do |e|
  puts [e.is_a?(StandardError), e.is_a?(ArgumentError), e.is_a?(KeyError), e.is_a?(IndexError), e.is_a?(RuntimeError), e.is_a?(Exception)].inspect
end
begin
  raise KeyError, "missing"
rescue StandardError => e
  puts e.is_a?(KeyError), e.is_a?(IndexError), e.kind_of?(ArgumentError), e.message
end

puts "-- class objects"
puts [Car.is_a?(Class), Car.is_a?(Module), Named.is_a?(Module), Named.is_a?(Class), Car.is_a?(Object)].inspect
puts [ident(Car).is_a?(Class), ident(Named).is_a?(Module), ident(Car).is_a?(Module), ident(1).is_a?(Class)].inspect

puts "-- namespaced classes"
pt3 = Geo::Pt3.new(2)
puts ident(Geo::Pt3.new(1)).is_a?(Geo::Pt), ident(Geo::Pt.new(1)).is_a?(Geo::Pt3), pt3.kind_of?(Geo::Pt), pt3.x

puts "-- narrowed untyped values take blocks"
puts walk([1, "a", nil, 2.5]), walk({ "k" => 1, :s => [2] }), walk(Geo::Pt3.new(7)), walk(3), walk([])

puts "-- untyped exceptions"
ex = ident(KeyError.new("k"))
puts ex.is_a?(IndexError), ex.is_a?(StandardError), ex.is_a?(ArgumentError), ex.is_a?(Exception), ex.kind_of?(KeyError)

puts "-- is_a? on self in a class method body is a run-time check"
puts Vehicle.new.car?, Car.new.car?, Truck.new.car?, Truck.new.self_truck?, Car.new.self_truck?

# Class values (decision 76): Module#===, is_a?/kind_of? with a class held in
# a variable, instance_of?, and === through untyped values.
cv_k = Car #: singleton(Vehicle)
puts [cv_k === Car.new, cv_k === Truck.new, Vehicle === Car.new, Named === Car.new, Named === Truck.new].inspect
puts [Integer === 3, Comparable === "s", String === :s, NilClass === nil, TrueClass === true, FalseClass === true].inspect
cv_e = KeyError.new("x")
puts [Exception === cv_e, StandardError === cv_e, IndexError === cv_e, ArgumentError === cv_e].inspect
cv_kinds = [KeyError, ArgumentError, IndexError] #: Array[singleton(StandardError)]
puts cv_kinds.map { |c| cv_e.is_a?(c) }.inspect, cv_kinds.map { |c| c === cv_e }.inspect
cv_m = Named #: Module
puts [Car.new.is_a?(cv_m), 3.kind_of?(cv_m)].inspect
puts [Car.new.instance_of?(Vehicle), Car.new.instance_of?(Car), 3.instance_of?(Integer), cv_e.instance_of?(IndexError)].inspect
puts cv_kinds.map { |c| ident(c) === cv_e }.inspect, cv_e.is_a?(ident(IndexError))
cv_vals = [1, "a", nil, :x, 2.0, [1], { a: 1 }] #: Array[untyped]
[Integer, String, NilClass, Symbol, Float, Array, Hash].each do |c|
  puts "#{c}: #{cv_vals.map { |v| c === v }.inspect}"
end
