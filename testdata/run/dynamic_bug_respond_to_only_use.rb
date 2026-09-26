# skip: a local whose only uses are respond_to? calls folded to constants fails go build ("declared and not used")

# rbs_inline: enabled

class Plain
  #: () -> String
  def hello = "hi"
end

p1 = Plain.new
puts p1.respond_to?(:hello), p1.respond_to?(:nope)
