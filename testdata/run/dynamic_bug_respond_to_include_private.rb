# rbs_inline: enabled

class Plain
  #: () -> String
  def hello = "hi"

  private

  #: () -> String
  def hidden = "hidden"
end

#: (untyped) -> untyped
def ident(v) = v

p1 = Plain.new
puts p1.respond_to?(:hidden, true), p1.respond_to?(:hello, true), p1.respond_to?(:nope, true), p1.hello
puts ident(p1).respond_to?(:hidden, true), ident(p1).respond_to?(:hidden)
