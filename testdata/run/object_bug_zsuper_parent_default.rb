# skip: zsuper (bare `super`) from a method with fewer parameters than the parent's optional ones does not fill the parent's defaults: go build fails "not enough arguments in call to Node_Initialize"

# rbs_inline: enabled

class Node
  attr_reader :id #: Integer

  #: (?Integer) -> void
  def initialize(id = 1)
    @id = id
  end
end

class Box < Node
  #: () -> void
  def initialize
    super
  end
end

puts Box.new.id.inspect
