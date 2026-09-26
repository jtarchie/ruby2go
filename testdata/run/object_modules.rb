# rbs_inline: enabled

# include, module methods over self, include order, super into a module, extend.

module Named
  #: () -> String
  def title = raise(NotImplementedError)

  #: () -> String
  def label = "[#{title}]"

  #: () -> String
  def shout = label.upcase
end

module Tagged
  #: () -> String
  def label = "tag:#{title}"

  #: () -> Integer
  def tag_len = title.size
end

module Greeter
  include Named

  #: (String) -> String
  def greet(other) = "#{label} greets #{other}"
end

class Doc
  include Named

  attr_reader :title #: String

  #: (String) -> void
  def initialize(title)
    @title = title
  end
end

class Both
  include Named
  include Tagged

  #: () -> String
  def title = "both"
end

class Override
  include Named

  #: () -> String
  def title = "ov"

  def label = "<#{super}>"
end

class Person
  include Greeter

  #: () -> String
  def title = "ann"
end

module Loud
  #: () -> String
  def hi = "loud"
end

class A
  #: () -> String
  def hi = "a"

  #: () -> String
  def base_only = "base"
end

class B < A
  include Loud
end

class C < B
  def hi = "c/" + super
end

class Q < Doc
end

module Registry
  extend Enumerable #[String]

  #: () { (String) -> void } -> void
  def self.each
    yield "alpha"
    yield "beta"
    yield "gamma"
  end
end

module Util
  #: (Integer) -> Integer
  def self.double(n) = n * 2

  #: (Integer) -> Integer
  def self.quad(n) = double(double(n))
end

d = Doc.new("readme")
puts d.label, d.shout
puts Both.new.label, Both.new.shout, Both.new.tag_len.inspect
puts Override.new.label, Override.new.shout
puts Person.new.greet("bob"), Person.new.shout
puts Doc.new("").label.inspect, Doc.new("ünï").shout

puts A.new.hi, B.new.hi, C.new.hi, C.new.base_only
puts Q.new("q").shout, Q.new("q").label

puts Registry.map { |s| s.upcase }.inspect
puts Registry.select { |s| s.include?("a") }.size.inspect
puts Registry.find { |s| s.start_with?("b") }.inspect
puts Registry.to_a.inspect, Registry.count.inspect, Registry.include?("beta").inspect
puts Registry.first(2).inspect, Registry.sort_by { |s| -s.size }.inspect

puts Util.double(21).inspect, Util.quad(-3).inspect, Util.quad(0).inspect
puts Util, Util.name.inspect, Util.class, Registry.inspect

# A module's own constants resolve lexically inside its methods; `-> self`, iterator,
# &block-forwarding and private methods in a module, reached from an includer and its subclass.
module Greeting
  PREFIX = "hi"

  #: () -> String
  def name = raise(NotImplementedError)

  #: () -> String
  def greet = "#{PREFIX} #{name}#{suffix}"

  #: () -> self
  def touch = self

  #: () { (Integer) -> void } -> void
  def each_two
    yield 1
    yield 2
  end

  #: (Integer) { (Integer) -> Integer } -> Array[Integer]
  def mapped(n, &blk) = [n, n + 1].map(&blk)

  private

  #: () -> String
  def suffix = "!"
end

class Member
  include Greeting

  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: () -> String
  def loud = greet.upcase + suffix
end

class Junior < Member
  def name = "jr"
end

pe = Member.new("ann")
jr = Junior.new("kid")
puts pe.greet, pe.touch.touch.name, pe.loud, jr.touch.greet, jr.loud
pe.each_two { |i| puts "two #{i}" }
jr.each_two do |i|
  next if i == 1
  puts "junior two #{i}"
end
puts pe.mapped(5) { |x| x * 10 }.inspect, jr.mapped(-1) { |x| x - 1 }.inspect
puts pe.is_a?(Greeting).inspect, jr.is_a?(Greeting).inspect, jr.is_a?(Member).inspect
puts pe.respond_to?(:greet).inspect, pe.respond_to?(:suffix).inspect
puts Greeting::PREFIX, Greeting.name, Greeting.inspect, Greeting.class
members = [pe, jr] #: Array[Member]
puts members.map(&:greet).inspect, members.map { |x| x.touch.name }.inspect
