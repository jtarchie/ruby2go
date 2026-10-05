# rbs_inline: enabled

# Class.new and Module.new with a block make a class at run time in Ruby.
# rb2go declares one class per literal at compile time instead.

# A constant names the class, as `class Point` would.
Point = Class.new do
  attr_reader :x #: Integer
  attr_reader :y #: Integer

  #: (Integer, Integer) -> void
  def initialize(x, y)
    @x = x
    @y = y
  end

  def to_s = "(#{x}, #{y})"
end

# A superclass goes in the parentheses.
NotFound = Class.new(StandardError)

Greeting = Module.new do
  #: (String) -> String
  def self.greet(name) = "hello, #{name}"
end

# A class held in a local is anonymous, as in MRI.
#: (Integer) -> Integer
def scaled(n)
  doubler = Class.new do
    #: (Integer) -> Integer
    def apply(v) = v * 2
  end
  doubler.new.apply(n)
end

puts Point.new(1, 2)
puts Point.name
puts Greeting.greet("robot")
puts scaled(21)

begin
  raise NotFound, "no such page"
rescue NotFound => e
  puts "#{e.class}: #{e.message}"
end

shout = Proc.new { "proc made with Proc.new" }
puts shout.call.upcase
