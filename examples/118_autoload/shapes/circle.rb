# rbs_inline: enabled

module Shapes
  class Circle
    #: (Integer) -> void
    def initialize(radius)
      @radius = radius
    end

    #: () -> String
    def name = "circle"

    #: () -> Float
    def area = Math::PI * @radius * @radius
  end
end
