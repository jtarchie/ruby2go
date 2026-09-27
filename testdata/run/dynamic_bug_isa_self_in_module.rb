# rbs_inline: enabled

module Named
  #: () -> String
  def tag = is_a?(Car) ? "car-named" : "named"

  #: () -> bool
  def vehicle? = kind_of?(Vehicle)
end

class Vehicle
end

class Car < Vehicle
  include Named
end

class Boat
  include Named
end

puts Car.new.tag, Boat.new.tag, Car.new.vehicle?, Boat.new.vehicle?
