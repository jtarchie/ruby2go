# rbs_inline: enabled

# Autoload registers a constant's file. MRI loads the file the first time the
# constant is named; rb2go compiles the whole program ahead of time, so the
# file is loaded where autoload registers it, as a require would be.
module Shapes
  autoload :Square, File.join(__dir__, "shapes", "square")
  autoload :Circle, File.expand_path("shapes/circle", __dir__)
end

shapes = [Shapes::Square.new(3), Shapes::Circle.new(2)] #: Array[Shapes::Square | Shapes::Circle]
shapes.each do |shape|
  puts "#{shape.name}: area #{shape.area.round(2)}"
end
