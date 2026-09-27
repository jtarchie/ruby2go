# rbs_inline: enabled

# a typed case/when on a primitive still checks the other primitive classes

#: (Integer) -> String
def typed_int(n)
  case n
  when Float then "float"
  when Integer then "int #{n + 1}"
  else "?"
  end
end

#: (String) -> String
def typed_str(s)
  case s
  when Symbol then "symbol"
  when String then "string #{s.size}"
  else "?"
  end
end

puts typed_int(4), typed_str("abc")

# case on a T? with `when nil` narrows the else branch to T
class Animal
  #: () -> String
  def name = "animal"
end

class Dog < Animal
  #: () -> String
  def bark = "woof"
end

#: (Animal?) -> String
def describe(a)
  case a
  when Dog then "dog #{a.bark}"
  when nil then "none"
  else "other #{a.name}"
  end
end

#: (String?) -> String
def text(s)
  case s
  when nil then "nil"
  else "text #{s.size}"
  end
end

puts describe(Dog.new), describe(nil), describe(Animal.new), text("abc"), text(nil)

# user methods named like generated dispatchers (plus/neg) must not collide
class Vec
  #: (Integer) -> Integer
  def plus(n) = n + 100

  #: () -> Integer
  def neg = -1
end

#: (untyped) -> untyped
def ident_dyn(v) = v

puts ident_dyn(1) + 2
puts ident_dyn(Vec.new).plus(1)
puts Vec.new.send(["neg"].first(1)[0] || "").inspect

# an untyped receiver passes Array[Integer]/Array[untyped] across generic params
class Foo
  #: (Array[untyped]) -> Integer
  def count_all(a) = a.size

  #: (Hash[Symbol, untyped]) -> Integer
  def opts(h) = h.size

  #: (Array[Integer]) -> Integer
  def total(a) = a.reduce(0) { |s, x| s + x }
end

#: (untyped) -> untyped
def ident_var(v) = v

foo_o = ident_var(Foo.new)
var_nums = [1, 2] #: Array[Integer]
var_mixed = [1, 2] #: Array[untyped]
puts foo_o.total(var_nums), foo_o.count_all(var_mixed)
puts foo_o.count_all(var_nums)
puts foo_o.opts({ k: 1 })
puts foo_o.total(var_mixed)
puts foo_o.total([])

# is_a?(Array) narrowing must mutate the caller's array, not a copy

#: (untyped) -> void
def push_if_array(v)
  v << 99 if v.is_a?(Array)
end

#: (untyped) -> void
def push_case(v)
  case v
  when Array then v << 7
  end
end

narrowed_a = [1, 2] #: Array[Integer]
push_if_array(narrowed_a)
push_case(narrowed_a)
puts narrowed_a.inspect

# is_a?/kind_of? on self inside a module method
module Named
  #: () -> String
  def tag = is_a?(Car) ? "car-named" : "named"

  #: () -> bool
  def vehicle? = kind_of?(Vehicle)
end

class Vehicle
end

class Car < Vehicle
  include Named
end

class Boat
  include Named
end

puts Car.new.tag, Boat.new.tag, Car.new.vehicle?, Boat.new.vehicle?

# method_missing/respond_to_missing? falling through to super
class Picky
  #: (Symbol, *untyped) -> String
  def method_missing(name, *args)
    return "ok #{name}" if name.to_s.start_with?("get_")
    super
  end

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("get_") || super
end

pk = Picky.new
puts pk.get_x, pk.respond_to?(:get_y), pk.respond_to?(:other)
begin
  pk.other
rescue NoMethodError => mm_err
  puts mm_err.message
end

# nil inside a rest arg reaches method_missing and a splat method intact
class Ghost
  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = "ghost #{name}(#{args.inspect})"
end

class Logger
  #: (*untyped) -> String
  def log(*parts) = parts.inspect
end

#: (untyped) -> untyped
def ident_rest(v) = v

puts ident_rest(Ghost.new).fly(1, nil)
puts ident_rest(Logger.new).log(nil, 2)

# nil elements of an untyped array survive is_a?/case narrowing

#: (untyped) -> untyped
def ident_opt(v) = v

#: (untyped) -> String
def nils(v)
  if v.is_a?(Array)
    v.map { |e| e.nil? }.inspect
  else
    "?"
  end
end

#: (untyped) -> String
def nils_case(v)
  case v
  when Array then v.select { |e| e.nil? }.size.to_s
  else "?"
  end
end

puts nils([1, nil, 3])
puts nils_case(["a", nil])
puts ident_opt([1, nil])[1].nil?

# calling a private method on an untyped value raises MRI's message
class Vault
  #: () -> String
  def open = "open"

  private

  #: () -> String
  def secret = "shh"
end

#: (untyped) -> untyped
def ident_priv(v) = v

vault_v = ident_priv(Vault.new)
puts vault_v.open
begin
  vault_v.secret
rescue NoMethodError => priv_err
  puts priv_err.message
end
begin
  vault_v.public_send(["secret"].first(1)[0] || "")
rescue NoMethodError => priv_err
  puts priv_err.message
end

# respond_to?(name, true) sees private methods, typed and untyped
class Plain
  #: () -> String
  def hello = "hi"

  private

  #: () -> String
  def hidden = "hidden"
end

#: (untyped) -> untyped
def ident_resp(v) = v

plain_p = Plain.new
puts plain_p.respond_to?(:hidden, true), plain_p.respond_to?(:hello, true), plain_p.respond_to?(:nope, true), plain_p.hello
puts ident_resp(plain_p).respond_to?(:hidden, true), ident_resp(plain_p).respond_to?(:hidden)

# respond_to? on self inside a module method depends on the includer
module NamedDrive
  #: () -> bool
  def can_drive? = respond_to?(:drive)

  #: () -> String
  def maybe_drive = respond_to?(:drive) ? "can drive" : "cannot"
end

class CarDrive
  include NamedDrive

  #: () -> String
  def drive = "vroom"
end

class BoatDrive
  include NamedDrive
end

puts CarDrive.new.can_drive?, BoatDrive.new.can_drive?, CarDrive.new.maybe_drive, BoatDrive.new.maybe_drive

# &. before an iterator call skips the block on nil
class Sides
  #: () { (Integer) -> void } -> void
  def each_side
    yield 1
    yield 2
  end
end

sides_h = { "a" => [1, 2] } #: Hash[String, Array[Integer]]
sides_h["a"]&.each { |x| puts x }
sides_h["b"]&.each { |x| puts x }
sides = nil #: Sides?
sides&.each_side { |s| puts "side #{s}" }
sides = Sides.new
sides&.each_side { |s| puts "side #{s}" }
puts "end"

# unlike public_send, send may call private methods on typed and untyped receivers
class VaultSend
  #: () -> String
  def open = "open"

  private

  #: () -> String
  def secret = "shh"
end

#: (untyped) -> untyped
def ident_send(v) = v

send_v = VaultSend.new
["open", "secret"].each { |meth| puts send_v.send(meth).inspect }
puts ident_send(send_v).send(:secret).inspect

# a typed String param reached through an untyped receiver raises TypeError
class Greeter
  #: (String) -> String
  def hi(other) = "hi " + other
end

#: (untyped) -> untyped
def ident_type(v) = v

greeter_g = ident_type(Greeter.new)
[5, 1.5, :sym, [1]].each do |bad|
  begin
    greeter_g.hi(bad)
  rescue TypeError => type_err
    puts type_err.message
  end
end

# != dispatches through a user == and a computed send of :!=
class VecNe
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (untyped) -> bool
  def ==(o) = o.is_a?(VecNe) && x == o.x
end

ne_m = [:!=, :==][0] #: Symbol?
ne_a = [1, 2] #: Array[Integer]
ne_v = VecNe.new(1) #: untyped
puts ne_a.send(ne_m || :x, [1, 2]).inspect
puts ne_v.send(ne_m || :x, VecNe.new(1)).inspect
puts (ne_v != VecNe.new(1)).inspect
ne_w = VecNe.new(1)
puts (ne_w != ne_v).inspect
ne_pair = [1, "a"] #: [Integer, String]
puts (ne_pair != [1, "a"]).inspect
ne_o = nil #: Integer?
ne_o = 3 if ne_a.size > 1
puts (ne_o != 3).inspect, (ne_o != nil).inspect
