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
