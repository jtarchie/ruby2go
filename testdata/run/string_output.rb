# rbs_inline: enabled

class Named
  #: () -> String
  def to_s = "named"
end

class Line
  #: () -> String
  def to_s = "ends\n"
end

class Custom
  #: () -> String
  def inspect = "<custom>"

  #: () -> String
  def to_s = "custom"
end

class Base
  #: () -> String
  def to_s = "base"
end

class Sub < Base
  #: () -> String
  def to_s = "sub:" + super
end

module Greets
  #: () -> String
  def to_s = "greets"
end

class Mixed
  include Greets
end

class LogStub
  #: (String) -> void
  def self.log(msg)
    puts "[log] #{msg}"
    print "[raw] ", msg, "\n"
  end

  #: () -> void
  def initialize
    puts "logger init"
  end

  #: () -> String
  def to_s = "a logger"

  #: () -> void
  def show
    puts self
    puts "#{self}!"
    [1, 2].each { |i| print i, self, "\n" }
  end
end

module Util
  #: (Integer) -> void
  def self.say(n) = puts(n * 2)
end

Pair = Struct.new(:a, :b) #: [Integer, String]
Val = Data.define(:v) #: [String]

#: (untyped) -> untyped
def ident(v) = v

class Speaker
  #: () -> void
  def hi
    self.puts "private puts through self"
    print "and print", "\n"
  end
end

# puts adds a newline only when the string lacks one.
puts
puts ""
puts "\n"
puts "a\n\n"
puts "x", "y"
puts "no double newline\n"

# puts calls to_s: numbers, booleans, symbols, nil.
puts 1, -2, 3.0, -0.0, 1e20, 1.5e-7, 12345678901234567.0, 2**40
puts true, false, :sym, nil
puts Named.new, Line.new, Custom.new

# Arrays are flattened one element per line; nested nil prints an empty line.
deep = [1, [2, [3]], [nil]] #: Array[untyped]
puts deep
puts ["a\n", "b"]
puts [[4], [5, 6]], 7, [nil]
words = ["w1", "w2"] #: Array[String]
puts words, words[5]

# print writes to_s with no separator and no newline.
print "p1", "p2", 3, nil, :s, 1.5, "\n"
print
print [1, "a", nil], "\n"
print "no newline yet"
print "\n"

# inspect on each core type
puts [1, "two", :three, nil, 2.5, true, [4]].inspect
puts({ "k" => 1, s: "v", 3 => nil }.inspect)
puts nil.inspect, 1.inspect, -1.5.inspect, true.inspect, :a.inspect, "s".inspect
puts Custom.new.inspect, [Custom.new].inspect, "#{Custom.new}"
puts nil.to_s.inspect, 12.to_s.inspect, 2.0.to_s.inspect, false.to_s.inspect
puts "abc".index("z").inspect, "abc".index("c").inspect, "abc"[9].inspect, "abc"[9].to_s.inspect

# to_s is dispatched virtually: a Base-typed Sub, an included module's to_s, class and module methods.
b = Sub.new #: Base
puts b, "#{b}", [b, Base.new].map(&:to_s).inspect
puts Mixed.new, "#{Mixed.new}"
LogStub.log("x")
LogStub.new.show
Util.say(21)
pr = Pair.new(1, "s")
puts pr, "#{pr}", Val.new("q")

# puts of hashes, symbol/float arrays and untyped values; print of a nil Integer?.
puts({ "k" => 1, b: [1, 2] })
puts [:a, :b], [1.5, 2.0]
arru = ident([1, [2, 3], "s"])
puts arru
puts "[#{ident(nil)}] #{arru}"
missing = [1][5]
print missing, "|", ident(nil), "|\n"

Speaker.new.hi

# puts flattens arrays and prints nil elements as blank lines
b_opt = ["x", nil] #: Array[String?]
puts b_opt
puts "after"
u_nested = [1, [2, nil]] #: untyped
puts u_nested
puts "end"

# puts [] prints a blank line, nested empties print nothing
puts "a"
puts []
puts "b"
puts [[], []]
puts "c"

# puts flattens nested arrays and tuples
puts [1, [2, [3]]]
tup = [1, "a"]
puts tup

# puts and print return nil; printf writes format's result
puts puts("a").inspect
puts print("b\n").nil?
printf("%d-%s\n", 1, "two")

# Output written before exit is flushed, and the status is kept.
puts "before exit"
print "no newline before exit"
exit 3
