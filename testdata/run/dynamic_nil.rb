# rbs_inline: enabled

class Box
  attr_reader :label #: String
  attr_reader :inner #: Box?

  #: (String, Box?) -> void
  def initialize(label, inner)
    @label = label
    @inner = inner
  end

  #: () -> String
  def to_s = "Box(#{label})"

  #: () -> Integer?
  def size_or_nil = label.empty? ? nil : label.size

  #: () -> String?
  def inner_label = inner&.label

  #: () -> Integer
  def depth = (inner&.depth || -1) + 1

  #: () -> void
  def shout
    puts "#{label.upcase}!"
  end
end

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

#: (Integer?) -> String
def describe(n)
  return "nothing" if n.nil?
  "got #{n + 0}"
end

class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (untyped) -> bool
  def ==(other) = other.is_a?(Pt) && other.x == x
end

#: (Integer) -> Pt?
def pt(n) = n.positive? ? Pt.new(n) : nil

#: (Integer) -> String?
def only_even(n)
  "even" if n.even?
end

#: (untyped) -> untyped
def ident(v) = v

class Lazy
  # @rbs @cache: String?
  # @rbs @count: Integer

  #: () -> void
  def initialize
    @cache = nil
    @count = 0
  end

  #: () -> String
  def value
    @cache ||= compute
  end

  #: () -> String
  def compute
    @count += 1
    "computed #{@count}"
  end

  #: () -> Integer?
  def cache_size = @cache&.size

  #: () -> String
  def peek = @cache || "empty"

  #: () -> void
  def reset
    @cache = nil
  end
end

puts "-- methods on nil and on a present T?"
none = maybe(-1)
some = maybe(3)
puts none.nil?.inspect, some.nil?.inspect
puts none.inspect, some.inspect
puts none.to_s.inspect, some.to_s.inspect
puts (none == nil).inspect, (some == nil).inspect, (none != nil).inspect, (some != 3).inspect
puts (some == 3).inspect, (none == 3).inspect, (some == 3.0).inspect
puts (!none).inspect, (!some).inspect
puts "[#{none}] [#{some}]"
puts describe(none), describe(some), describe(0), describe(maybe(0))
puts nil.inspect, nil.to_s.inspect, nil.nil?.inspect, (nil == nil).inspect, "#{nil}|"

puts "-- puts of nil prints an empty line"
puts none
puts nil

puts "-- &. on nil and present values"
b = Box.new("outer", Box.new("mid", Box.new("", nil)))
puts b.inner&.label.inspect
puts b.inner&.inner&.label.inspect
puts b.inner&.inner&.inner&.label.inspect
puts b.inner&.inner&.inner&.inner&.label.inspect
puts b.inner&.size_or_nil.inspect
puts b.inner&.inner&.size_or_nil.inspect
b.inner&.shout
b.inner&.inner&.inner&.shout
puts b.inner&.to_s.inspect
nobox = nil #: Box?
puts nobox&.label.inspect, nobox&.inner&.label.inspect, nobox&.size_or_nil.inspect
nobox&.shout
str = nil #: String?
puts str&.upcase.inspect, str&.size.inspect, str&.empty?.inspect
str = "héllo"
puts str&.upcase.inspect, str&.size.inspect, str&.empty?.inspect

puts b.inner_label.inspect, b.inner&.inner&.inner_label.inspect, b.depth, Box.new("x", nil).depth
puts "has inner" if b.inner&.label
puts "no inner-inner-inner" unless b.inner&.inner&.inner
puts str&.center(9).inspect, nobox&.label&.center(9).inspect

puts "-- == and != between T? values compare the values"
puts (maybe(1) == maybe(1)).inspect, (maybe(-1) == maybe(-2)).inspect, (maybe(1) == maybe(-1)).inspect, (maybe(2) != maybe(3)).inspect
puts (maybe(-1) != nil).inspect, (nil == maybe(-1)).inspect, (maybe(7) == 7).inspect, (maybe(7) != 7.0).inspect

puts "-- || picks the first non-nil"
h = { "a" => 1, "z" => 0 } #: Hash[String, Integer]
puts (h["a"] || -1).inspect, (h["b"] || -1).inspect, (h["z"] || -1).inspect
puts (none || some || 99).inspect, (none || maybe(-5) || 99).inspect
puts (str || "default").inspect
empty = nil #: String?
puts (empty || "").inspect, (empty || "d").size
puts (b.inner&.inner&.inner&.label || "none").inspect
puts (nobox&.label || "none").inspect

puts "-- ||= assigns only when nil"
cache = nil #: Integer?
cache ||= 10
puts cache.inspect
cache ||= 20
puts cache.inspect
label = nil #: String?
label ||= "x" * 3
label ||= "never"
puts label.inspect
zero = 0 #: Integer?
zero ||= 5
puts zero.inspect

puts "-- && yields the right side or nil"
v = some && some * 2
puts v.inspect
w = none && none * 2
puts w.inspect
flag = some && some > 2
puts flag.inspect

puts "-- nil in collections"
nums = [3, 1, 2] #: Array[Integer]
found = nums.find { |x| x > 2 }
missing = nums.find { |x| x > 5 }
puts found.inspect, missing.inspect, (found || 0) + (missing || 0)
puts h["a"].inspect, h["missing"].inspect, h.fetch("missing", 7).inspect
none_list = [] #: Array[Integer]
puts none_list.first(1).inspect, none_list.min.inspect, none_list.max.inspect, none_list.pop.inspect
puts nums.detect { |x| x == 9 }.inspect
maybes = [maybe(1), maybe(-1), maybe(2)] #: Array[Integer?]
puts maybes.size, maybes.select { |m| m.nil? }.size.inspect
puts maybes.map { |m| m.nil? }.inspect, maybes.map { |m| m.inspect }.inspect, maybes.map { |m| m.to_s }.inspect
puts maybes.map { |m| (m || 0) * 10 }.inspect

puts "-- ternary on T?"
puts (some ? some + 1 : 0).inspect, (none ? none + 1 : 0).inspect

puts "-- &. on method results, with arguments, chained into ||"
fruit = { "a" => "apple", "n" => "" } #: Hash[String, String]
puts fruit["a"]&.upcase.inspect, fruit["zz"]&.upcase.inspect, fruit["n"]&.empty?.inspect
puts fruit["a"]&.size&.to_s.inspect, fruit["zz"]&.size&.to_s.inspect, (fruit["zz"]&.size || 0) + 1
puts fruit["a"]&.center(9).inspect, fruit["zz"]&.center(9).inspect
lists = { "a" => [1, 2, 3] } #: Hash[String, Array[Integer]]
puts lists["a"]&.map { |x| x * 2 }.inspect, lists["b"]&.map { |x| x * 2 }.inspect, lists["a"]&.select { |x| x.odd? }&.size.inspect
w2 = nil #: Integer?
[1, nil, 3].each do |x|
  w2 = x
  puts w2&.+(1).inspect
end

puts "-- T? of a struct class: == uses the class's ==, round trips through untyped"
puts (pt(1) == pt(1)).inspect, (pt(1) == pt(2)).inspect, (pt(0) == pt(-1)).inspect, (pt(1) == nil).inspect, (pt(0) == nil).inspect
puts ident(pt(3)).x, ident(pt(0)).inspect, (pt(4)&.x || 0), pt(0)&.x.inspect
op = pt(0)
puts(op ? op.x : -1)

puts "-- an if without else yields nil"
puts only_even(2).inspect, only_even(3).inspect, (only_even(5) || "odd")

puts "-- respond_to? on a T? asks the value, or nil"
present = maybe(4)
puts present.respond_to?(:abs).inspect, none.respond_to?(:abs).inspect, none.respond_to?(:nil?).inspect, none.respond_to?(:inspect).inspect

puts "-- T? instance variables: ||= memoizes, &. and || read them"
lz = Lazy.new
puts lz.cache_size.inspect, lz.peek
puts lz.value, lz.value, lz.cache_size.inspect, lz.peek
lz.reset
puts lz.value, lz.peek

puts "-- calling a method on nil raises NoMethodError"
begin
  puts none.abs
rescue NoMethodError => e
  puts "NoMethodError: #{e.message}"
end
puts some.abs
begin
  puts empty.upcase
rescue NoMethodError => e
  puts "NoMethodError: #{e.message}"
end
puts "done"
puts "-- uncaught NoMethodError on nil exits 1 after flushing stdout"
puts empty.size
puts "not reached"
