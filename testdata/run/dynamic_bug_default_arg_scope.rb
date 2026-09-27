# rbs_inline: enabled

class Foo
  #: () -> void
  def initialize
    @x = 5
  end

  #: (String, ?Integer) -> Integer
  def f(a, b = a.size) = b

  #: (?Integer) -> Integer
  def get(a = @x) = a

  #: (?String) -> String
  def greet(g = greeting) = g + "!"

  #: () -> String
  def greeting = "hi from Foo"
end

class Bar
  #: () -> void
  def initialize
    @x = 99
  end

  #: () -> String
  def greeting = "hi from Bar"

  #: (Foo) -> String
  def ask(foo) = "#{foo.get} #{foo.greet}"
end

#: (untyped) -> untyped
def ident(v) = v

puts Bar.new.ask(Foo.new)
puts Foo.new.f("typed")
puts ident(Foo.new).f("abc")
puts ident(Foo.new).f("abc", 7)
