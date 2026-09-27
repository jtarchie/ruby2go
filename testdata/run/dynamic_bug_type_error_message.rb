# rbs_inline: enabled

class Greeter
  #: (String) -> String
  def hi(other) = "hi " + other
end

#: (untyped) -> untyped
def ident(v) = v

g = ident(Greeter.new)
[5, 1.5, :sym, [1]].each do |bad|
  begin
    g.hi(bad)
  rescue TypeError => e
    puts e.message
  end
end
