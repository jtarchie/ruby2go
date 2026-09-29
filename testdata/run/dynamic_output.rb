# rbs_inline: enabled

class Temp
  attr_reader :deg #: Integer

  #: (Integer) -> void
  def initialize(deg)
    @deg = deg
  end

  #: () -> String
  def to_s = "#{deg}°"

  #: () -> String
  def inspect = "#<Temp #{deg}>"
end

#: (untyped) -> untyped
def ident(v) = v

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

puts "-- puts of nil prints an empty line"
none = maybe(-1)
puts none
puts nil

puts "-- puts of untyped values"
puts ident(nil)
puts ident([1, ["", "x"]])
puts ident(Temp.new(3))
puts ident(:sym), ident(1.0), ident(false)
