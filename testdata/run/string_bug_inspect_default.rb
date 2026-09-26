# skip: Kernel#inspect is to_s; MRI default inspect is #<Class ...ivars> regardless of to_s

# rbs_inline: enabled

class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: () -> String
  def to_s = "Pt(#{x})"
end

puts (Pt.new(3).inspect == Pt.new(3).to_s).inspect
puts Pt.new(3).inspect.include?("@x=3").inspect
