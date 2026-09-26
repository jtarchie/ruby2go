# skip: `super` into an attr_reader/attr_writer (overriding an accessor) calls a free function Base_Name that is never emitted, so go build fails "undefined: Base_Name"

# rbs_inline: enabled

class Base
  attr_accessor :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end
end

class Loud < Base
  def name = super.upcase

  def name=(v)
    super(v + "!")
  end
end

l = Loud.new("ann")
puts l.name
l.name = "bob"
puts l.name
