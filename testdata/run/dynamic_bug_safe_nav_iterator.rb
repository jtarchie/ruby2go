# rbs_inline: enabled

class Sides
  #: () { (Integer) -> void } -> void
  def each_side
    yield 1
    yield 2
  end
end

h = { "a" => [1, 2] } #: Hash[String, Array[Integer]]
h["a"]&.each { |x| puts x }
h["b"]&.each { |x| puts x }
sides = nil #: Sides?
sides&.each_side { |s| puts "side #{s}" }
sides = Sides.new
sides&.each_side { |s| puts "side #{s}" }
puts "end"
