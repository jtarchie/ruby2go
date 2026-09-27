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
