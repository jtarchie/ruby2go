# rbs_inline: enabled

module Loud
  #: () -> String
  def hi = "loud " + super
end

class A
  #: () -> String
  def hi = "a"
end

class B < A
  include Loud
end

puts B.new.hi

# The target follows the module in each includer's ancestors: another
# module, the includer's superclass, or nothing (NoMethodError).
module Shout
  #: (Integer) -> Integer
  def calc(x) = super(x + 1) * 2

  #: (Integer) -> void
  def initialize(n)
    super
    puts "shout init #{n}"
  end

  #: () -> String
  def hi = "shout " + super
end

module Quiet
  #: () -> String
  def hi = "quiet " + super
end

module Both
  include Shout

  #: () -> String
  def hi = "both " + super
end

class P
  #: (Integer) -> void
  def initialize(n)
    @n = n
    puts "p init #{n}"
  end

  #: () -> String
  def hi = "p#{@n}"

  #: (Integer) -> Integer
  def calc(x) = x + 100
end

class Q < P
  include Shout
end

class R < Q
  #: () -> String
  def hi = "r " + super
end

class S < P
  include Shout
  include Quiet
end

class T < P
  include Both
end

class U
  include Quiet
end

puts Q.new(1).hi
puts Q.new(2).calc(3)
puts R.new(4).hi
puts S.new(5).hi
puts T.new(6).hi
begin
  U.new.hi
rescue NoMethodError => e
  puts e.message
end
