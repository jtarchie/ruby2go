# skip: respond_to?(:m) on self inside a module method folds to false when the module lacks m, even if the includer defines it; MRI asks the receiver's class

# rbs_inline: enabled

module Named
  #: () -> bool
  def can_drive? = respond_to?(:drive)

  #: () -> String
  def maybe_drive = respond_to?(:drive) ? "can drive" : "cannot"
end

class Car
  include Named

  #: () -> String
  def drive = "vroom"
end

class Boat
  include Named
end

puts Car.new.can_drive?, Boat.new.can_drive?, Car.new.maybe_drive, Boat.new.maybe_drive
