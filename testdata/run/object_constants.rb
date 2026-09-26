# rbs_inline: enabled

# Module#constants, #const_get (symbols, strings, paths, top-level fallback, typed results), #const_defined?, NameError.

module Plugins
  VERSION = "1.0"
  MAX = 3

  class Base
    #: () -> String
    def self.id = "base"

    #: () -> String
    def run = "base run"
  end

  class Alpha < Base
    def self.id = "alpha"

    def run = "alpha run"
  end

  class Beta < Base
    def self.id = "beta"
  end
end

module Kinds
  class Root
    #: () -> Integer
    def self.rank = 0
  end

  class One < Root
    def self.rank = 1
  end

  class Two < Root
    def self.rank = 2
  end
end

class Parent
  COLOR = "red"
end

class Child < Parent
  SIZE = 5
end

TOP = 99

#: (String) -> Integer
def rank_of(name) = Kinds.const_get(name).rank

puts Plugins.constants.sort.inspect, Kinds.constants.sort.inspect
puts Child.constants.sort.inspect, Parent.constants.inspect
puts Plugins::Alpha.constants.inspect

puts Plugins.const_get(:VERSION), Plugins.const_get("MAX").inspect, (Plugins.const_get(:MAX) + 1).inspect
puts Plugins.const_get(:Alpha).id, Plugins.const_get("Beta").new.run, Plugins.const_get(:Alpha).new.run
puts rank_of("One").inspect, rank_of("Two").inspect, rank_of("Root").inspect
puts Kinds.constants.sort.map { |c| Kinds.const_get(c).rank }.inspect
puts Child.const_get(:COLOR), Child.const_get(:SIZE).inspect
puts Object.const_get(:TOP).inspect, Object.const_get("Plugins::Alpha"), Object.const_get("::Plugins::VERSION")
puts Plugins.const_get(:TOP).inspect, Plugins.const_get("String"), Object.const_get(:Plugins) == Plugins
puts Plugins.const_get("Alpha") == Plugins::Alpha, Plugins.const_get(:Beta) == Plugins::Alpha

puts Plugins.const_defined?(:VERSION).inspect, Plugins.const_defined?("Alpha").inspect, Plugins.const_defined?(:Gamma).inspect
puts Child.const_defined?(:COLOR).inspect, Object.const_defined?("Plugins::Base").inspect, Object.const_defined?("Plugins::Nope").inspect
puts Plugins.const_defined?(:TOP).inspect, Object.const_defined?(:TOP).inspect

begin
  Plugins.const_get(:Gamma)
rescue NameError => e
  puts "NameError: #{e.message}"
end

begin
  Object.const_get("Nowhere")
rescue NameError => e
  puts "NameError: #{e.message}"
end

begin
  Object.const_get("Plugins::Alpha::Deep")
rescue NameError => e
  puts "NameError: #{e.message}"
end

# A computed name on Child joins its own and inherited constants (Integer + String: untyped);
# same-typed constants join to their type; Struct/Data classes are constants too.
#: (String) -> untyped
def child_const(n) = Child.const_get(n)

module Limits
  LOW = 1
  HIGH = 9
end

module Geo
  Coord = Data.define(:lat, :lng) #: [Float, Float]
  Tag = Struct.new(:text) #: [String]
  ORIGIN = Coord.new(0.0, 0.0)
end

puts child_const("SIZE").inspect, child_const("COLOR").inspect, Child.const_get("SIZE".downcase.upcase).inspect
puts Limits.constants.sort.map { |c| Limits.const_get(c) * 2 }.inspect
puts Geo.constants.sort.inspect, Geo.const_get(:Tag).new("t").inspect, Geo.const_get(:Coord).members.inspect
puts Geo.const_get(:ORIGIN).lat.inspect, Geo::Tag.constants.inspect, Geo.const_get("Coord").name

puts "before"
Kinds.const_get(:Three)
puts "unreachable"
