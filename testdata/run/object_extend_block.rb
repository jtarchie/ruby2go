# rbs_inline: enabled

# extend M on classes and modules (inherited by subclasses), extend Enumerable over self.each, &block forwarding.

module Describable
  #: (Integer) -> Integer
  def twice(n) = n * 2

  #: () -> String
  def describe = "I am #{name}"
end

class Widget
  extend Describable

  #: () -> String
  def self.kind = "widget"
end

class Gadget < Widget
end

module Tools
  extend Describable
end

module Colors
  extend Enumerable #[String]

  RED = "red"
  GREEN = "green"
  BLUE = "blue"

  #: () { (String) -> void } -> void
  def self.each(&block)
    [RED, GREEN, BLUE].each(&block)
  end
end

module Handlers
  extend Enumerable #[singleton(Handlers::Base)]

  class Base
    #: (String) -> bool
    def self.matches?(path) = false
  end

  class Posts < Base
    def self.matches?(path) = path.start_with?("/posts")
  end

  class Users < Base
    def self.matches?(path) = path.start_with?("/users")
  end

  #: () { (singleton(Base)) -> void } -> void
  def self.each(&block)
    constants.sort.map { |c| const_get(c) }.each(&block)
  end
end

class Bag
  #: () -> void
  def initialize
    @items = [] #: Array[Integer]
  end

  #: (Integer) -> self
  def add(n)
    @items << n
    self
  end

  #: () { (Integer) -> void } -> void
  def each(&block) = @items.each(&block)

  #: () { (Integer) -> Integer } -> Array[Integer]
  def transform(&block) = @items.map(&block)

  #: () { (Integer) -> bool } -> Array[Integer]
  def keep(&block) = @items.select(&block)

  #: () { (Integer) -> String } -> String
  def first_as(&block) = block.call(@items.first(1).fetch(0))

  #: () { (Integer) -> void } -> void
  def each_twice(&block)
    each(&block)
    each(&block)
  end
end

puts Widget.twice(4).inspect, Gadget.twice(-1).inspect, Tools.twice(0).inspect
puts Widget.describe, Gadget.describe, Tools.describe, Gadget.kind

puts Colors.map { |c| c.upcase }.inspect, Colors.count.inspect, Colors.to_a.inspect
puts Colors.select { |c| c.include?("e") }.inspect, Colors.find { |c| c.size == 4 }.inspect
puts Colors.include?("green").inspect, Colors.include?("pink").inspect, Colors.first(1).inspect
puts Colors.sort_by { |c| c.size }.inspect, Colors.min.inspect, Colors.max.inspect

puts Handlers.detect { |h| h.matches?("/users/1") }.inspect
puts Handlers.map { |h| h.name }.inspect, Handlers.count.inspect
puts Handlers.select { |h| h.matches?("/posts/9") }.inspect, Handlers.find { |h| h.matches?("/x") }.inspect

bag = Bag.new.add(3).add(1).add(2)
bag.each { |n| puts n }
puts bag.transform { |n| n * n }.inspect, bag.keep { |n| n.odd? }.inspect, bag.first_as { |n| "first=#{n}" }
total = 0
bag.each_twice { |n| total += n }
puts total.inspect
bag.each do |n|
  next if n == 1
  puts "saw #{n}"
end

# A class method overrides a method its class got from `extend`; the extended method
# dispatches back to the override, per subclass, also through singleton(C).
module Labelled
  #: () -> String
  def label = "#{name}:#{kind}"

  #: () -> String
  def kind = "base"
end

class Knob
  extend Labelled

  def self.kind = "knob"
end

class BigKnob < Knob
  def self.kind = "big"
end

class Lever
  extend Labelled
end

puts Knob.label, BigKnob.label, Lever.label, Lever.kind
knobs = [Knob, BigKnob] #: Array[singleton(Knob)]
puts knobs.map(&:label).inspect, knobs.map { |k| k.kind }.inspect
