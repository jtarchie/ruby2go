# skip: is_a?/kind_of?(C) on self inside a module method folds to false (the module is not a subclass of C), so an includer that is a C answers false; MRI checks the receiver's class

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
