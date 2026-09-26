# rbs_inline: enabled

class Temp
  attr_reader :deg #: Integer

  #: (Integer) -> void
  def initialize(deg)
    @deg = deg
  end

  #: () -> String
  def to_s = "#{deg}°"

  #: () -> String
  def inspect = "#<Temp #{deg}>"
end

#: (untyped) -> untyped
def ident(v) = v

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

#: (untyped) -> String
def truth(v)
  if v
    "truthy"
  else
    "falsy"
  end
end

#: (untyped) -> String
def kind(v)
  case v
  when nil then "nil"
  when String then "String"
  else "other"
  end
end

#: (String) -> Integer
def strlen(s) = s.size

#: (untyped?) -> String
def show(v) = v.inspect

#: (untyped?) -> untyped?
def pass_on(v) = v

class Holder
  attr_reader :value #: untyped

  #: (untyped) -> void
  def initialize(value)
    @value = value
  end

  #: () -> String
  def describe
    if value.is_a?(String)
      "str #{value.upcase}"
    elsif value.is_a?(Integer)
      "int #{value + 1}"
    elsif value.nil?
      "nil"
    else
      "other #{value.inspect}"
    end
  end

  #: () -> String
  def via_case
    case value
    when String then "S#{value.size}"
    when Array then "A#{value.size}"
    else "?"
    end
  end

  #: () -> String
  def ivar_dyn = "#{@value.to_s}!"
end

#: (bool) -> String
def yn(b) = b ? "y" : "n"

#: (Integer?) -> String
def opt_desc(n) = n.nil? ? "none" : "n=#{n}"

#: (Array[Integer]) -> Integer
def total(a) = a.reduce(0) { |s, x| s + x }

#: (String) -> String
def up(s) = s.upcase

vals = [nil, false, true, 0, 0.0, -1, "", "0", :a, [], {}, Temp.new(-4)] #: Array[untyped]

puts "-- truthiness: only nil and false are falsy"
vals.each { |v| puts "#{v.inspect} #{truth(v)} #{(!v).inspect} #{v ? 1 : 2}" }
puts vals.select { |v| v }.size, vals.reject { |v| v }.size

puts "-- to_s, inspect and interpolation of untyped values"
more = [2**40, -0.5, 1e20, 0.1 + 0.2, "tab\there", "quote\"s", "uni ✓", "", :"with space", ["n", [1, [2]]], { "k" => "v", :s => 1.0 }, Temp.new(21)] #: Array[untyped]
more.each { |v| puts v.inspect, v.to_s, "<#{v}>" }
puts more.inspect
puts vals.inspect

puts "-- puts of untyped values"
puts ident(nil)
puts ident([1, ["", "x"]])
puts ident(Temp.new(3))
puts ident(:sym), ident(1.0), ident(false)

puts "-- || and && on untyped"
puts (ident(nil) || "left nil").inspect, (ident(false) || 0).inspect, (ident(0) || 1).inspect, (ident("") || "x").inspect
puts (ident(nil) && 1).inspect, (ident(false) && 1).inspect, (ident(0) && "right").inspect, (ident([]) && ident(nil)).inspect
x = nil #: untyped
x ||= 5
puts x.inspect
y = ident(false)
y ||= "was false"
puts y.inspect
z = ident(0)
z ||= 99
puts z.inspect

puts "-- ==, != and nil? on untyped"
puts (ident(1) == 1).inspect, (ident(1) == 1.0).inspect, (ident("a") == "a").inspect, (ident(:a) == "a").inspect
puts (ident(nil) == nil).inspect, (ident(false) == nil).inspect, (ident([1, 2]) == [1, 2]).inspect, (ident(1) != 2).inspect
puts ident(nil).nil?.inspect, ident(false).nil?.inspect, ident(0).nil?.inspect
puts (1 == ident(1)).inspect, ("b" == ident("a")).inspect

puts (ident(:a).equal?(:a)).inspect, (ident(1).equal?(1)).inspect, (ident(nil).equal?(nil)).inspect, (ident(1).equal?(2)).inspect

puts "-- T? crosses into untyped as nil or the value"
puts kind(maybe(-1)), kind(maybe(2)), kind(ident(maybe(-3)))
puts ident(maybe(5)).inspect, ident(maybe(-5)).inspect, truth(maybe(-1)), truth(maybe(1))

puts "-- untyped passes into typed parameters"
puts strlen(ident("héllo")), strlen(ident(""))
n = ident(41) #: Integer
puts n + 1
str = ident("s") #: String
puts str * 3
opt = ident(nil) #: String?
puts opt.inspect, (opt || "d").inspect

puts "-- untyped? is untyped"
puts show(nil), show(3), show("héllo"), pass_on(nil).inspect, pass_on("x").inspect, truth(pass_on(false))

puts "-- untyped collections"
arr = [1, "two", :three, 4.0, nil] #: Array[untyped]
puts arr.size, arr.first(2).inspect, arr.last.inspect, arr.map { |e| e.to_s }.inspect, arr.compact.size
arr << [5]
puts arr.inspect
h = { "a" => 1, "b" => "bee", "c" => nil } #: Hash[String, untyped]
puts h.inspect, h["a"].inspect, h["b"].inspect, h["c"].inspect, h["zz"].inspect
puts h.fetch("b", 0).inspect, h.fetch("zz", 0).inspect, h.keys.inspect
h.each { |k, v| puts "#{k}=#{v.inspect} #{truth(v)}" }
empty = [] #: Array[untyped]
puts empty.inspect, empty.first(1).inspect, empty.last.inspect

puts "-- untyped attributes narrow like locals"
[Holder.new("abc"), Holder.new(41), Holder.new(nil), Holder.new([1, 2]), Holder.new(:s)].each do |hd|
  puts hd.describe, hd.via_case, hd.ivar_dyn
end

puts "-- Hash[Symbol, untyped] as configuration"
cfg = { port: 8080, host: "localhost", tags: ["a", "b"] } #: Hash[Symbol, untyped]
puts (cfg[:port] || 80) + 1, cfg[:host].upcase, cfg[:tags].size, (cfg[:missing] || "dflt")
puts "has host" if cfg[:host]
puts "no missing" unless cfg[:missing]

puts "-- untyped into bool, T? and generic parameters"
puts yn(ident(nil)), yn(ident(false)), yn(ident(0)), yn(ident("")), yn(ident(true))
puts opt_desc(ident(nil)), opt_desc(ident(4)), total(ident([1, 2, 3]))
begin
  up(ident(5))
rescue StandardError
  puts "a failed assertion raises a StandardError"
end
