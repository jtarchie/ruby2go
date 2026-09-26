# skip: an annotated override whose signature differs from the parent's (other arity, or a narrower return type Sub for Base) fails go build "*Sub does not implement BaseI (wrong type for method F)"; Ruby allows both

# rbs_inline: enabled

class Base
  #: (Integer) -> String
  def f(n) = "base #{n}"

  #: () -> Base
  def me = self

  #: () -> String
  def name = "base"
end

class Sub < Base
  #: () -> String
  def f = "sub"

  #: () -> Sub
  def me = self

  def name = "sub"

  #: () -> String
  def only = "only"
end

puts Base.new.f(1), Sub.new.f
puts Base.new.me.name, Sub.new.me.name, Sub.new.me.only
