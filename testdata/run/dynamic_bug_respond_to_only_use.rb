# rbs_inline: enabled

class Plain
  #: () -> String
  def hello = "hi"
end

p1 = Plain.new
puts p1.respond_to?(:hello), p1.respond_to?(:nope)
