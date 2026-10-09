# rbs_inline: enabled

module Shapes
  class Square
    #: (Integer) -> void
    def initialize(side)
      @side = side
    end

    #: () -> String
    def name = "square"

    #: () -> Integer
    def area = @side * @side
  end
end
