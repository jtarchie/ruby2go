# rbs_inline: enabled

# Differing override signatures must survive three levels and every call path (parent type, own type, super, untyped).

class Base
  #: (Integer) -> String
  def f(n) = "base #{n}"

  #: () -> Base
  def me = self

  #: () -> self
  def dup_me = self

  #: (self) -> bool
  def same?(o) = o.equal?(self)

  #: () -> String
  def name = "base"

  #: () -> String
  def call_f = f(7)

  #: (String) -> String
  def self.make(s) = "Base.make #{s}"
end

class Sub < Base
  #: (?Integer) -> String
  def f(n = 3) = "sub #{n} " + super(n)

  #: () -> Sub
  def me = self

  def dup_me = self

  def same?(o) = !o.equal?(self)

  def name = "sub"

  #: () -> String
  def only = "only"

  #: () -> String
  def self.make = "Sub.make"
end

class SubSub < Sub
  #: () -> String
  def f = "subsub"

  def name = "subsub"
end

class Leaf < SubSub
end

#: (Base) -> String
def show(b)
  "#{b.name} #{b.me.name} #{b.dup_me.name}"
end

puts Base.new.f(1), Sub.new.f, Sub.new.f(5), SubSub.new.f, Leaf.new.f
puts Sub.new.call_f
[Base.new, Sub.new, SubSub.new, Leaf.new].each { |b| puts show(b) }
s = Sub.new
puts s.same?(s), Base.new.same?(s)
puts Leaf.new.me.only, Leaf.new.dup_me.only
x = Sub.new #: untyped
puts x.f(2), x.f
puts Base.make("x"), Sub.make
k = Sub #: singleton(Base)
begin
  puts k.make("y")
rescue ArgumentError => e
  puts "#{e.class}: #{e.message}"
end
begin
  SubSub.new.call_f
rescue ArgumentError => e
  puts "#{e.class}: #{e.message}"
end
