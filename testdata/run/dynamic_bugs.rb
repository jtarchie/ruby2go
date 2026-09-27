# rbs_inline: enabled

# case/when Object matches anything but nil

#: (untyped) -> String
def f(v)
  case v
  when Object then "object"
  else "not an object"
  end
end
puts f(1), f(nil), f(false)

# is_a?/kind_of? across a user subclass, a temp const and builtins
class Vehicle
end

class Car < Vehicle
end
puts :s.is_a?(Symbol), [1].is_a?(Array), { "a" => 1 }.is_a?(Hash), Car.new.is_a?(Vehicle), Car.new.kind_of?(String)

# is_a?(String) narrows an untyped result before a typed call

#: (Integer) -> String?
def word(n) = n.positive? ? "w" * n : nil

#: (String) -> Integer
def len(s) = s.size

s = word(2)
puts len(s) if s.is_a?(String)
t = word(0)
puts (t.is_a?(String) ? len(t) : -1).inspect

# is_a?(Object)/kind_of?(BasicObject) on nil

#: (Integer) -> Integer?
def maybe_isa(n) = n.positive? ? n : nil

none = maybe_isa(-1)
puts none.is_a?(Object).inspect, none.kind_of?(BasicObject).inspect, none.is_a?(Kernel).inspect
puts none.is_a?(Integer).inspect, maybe_isa(2).is_a?(Object).inspect

# a public method_missing is still respond_to?-visible
class Config
  #: (Symbol, *untyped) -> String
  def method_missing(name, *args) = "mm #{name}"

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = false
end

c = Config.new
puts c.respond_to?(:method_missing), c.respond_to?(:respond_to_missing?)

# Array[Integer?] element equality and include? against a boxed nil

#: (Integer) -> Integer?
def maybe_elem(n) = n.positive? ? n : nil

elems = [maybe_elem(1), maybe_elem(2)] #: Array[Integer?]
puts (elems == [maybe_elem(1), maybe_elem(2)]).inspect
puts elems.include?(maybe_elem(2)).inspect
puts elems.include?(2).inspect

# respond_to? on an ordinary typed method, present and absent
class Plain
  #: () -> String
  def hello = "hi"
end

p1 = Plain.new
puts p1.respond_to?(:hello), p1.respond_to?(:nope)

# &. on untyped values: nil, a string, hash/array elements

#: (untyped) -> untyped
def ident_safe(v) = v

u = ident_safe(nil)
puts u&.size.inspect
v = ident_safe("abc")
puts v&.size.inspect
h = { "n" => nil, "s" => "str" } #: Hash[String, untyped]
puts h["s"]&.size.inspect, h["zz"]&.size.inspect
puts h["n"]&.size.inspect
a = [nil, "x"] #: Array[untyped]
puts a[0]&.size.inspect

# .class/.class.name on untyped values, including a user class
class Foo
end

#: (untyped) -> untyped
def ident_class(v) = v

puts ident_class(1).class, ident_class("s").class.name, ident_class(Foo.new).class, ident_class([1]).class, ident_class(nil).class
puts "#{ident_class(2.5).class}"

# untyped? parameter, boxed nil vs. a value

#: (Integer) -> Integer?
def maybe_opt(n) = n.positive? ? n : nil

#: (untyped?) -> String
def show(v) = v.inspect

puts show(maybe_opt(-1)), show(maybe_opt(4))
