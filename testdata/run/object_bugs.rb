# rbs_inline: enabled

# user classes named like runtime helpers (Opt, Ref) must not clash with them
class Opt
  #: () -> String
  def to_s = "opt"
end

class Ref
  #: () -> String
  def to_s = "ref"
end

puts Opt.new, Ref.new

# a constant read before its assignment raises NameError, like MRI

#: () -> Integer
def lim = LIMIT

begin
  puts lim
rescue NameError => name_err
  puts "NameError: #{name_err.message}"
end
LIMIT = 5
puts lim

# const_get on an optional module constant keeps its nil
module Settings
  DEFAULT = nil #: String?
end

puts Settings::DEFAULT.inspect, Settings.constants.inspect
puts Settings.const_get(:DEFAULT).inspect
puts Settings.const_get("DEFAULT").nil?.inspect

# const_get through a non-module value raises TypeError
module Plugins
  VERSION = "1.0"
end

begin
  Object.const_get("Plugins::VERSION::X")
rescue TypeError => type_err
  puts "TypeError: #{type_err.message}"
end

# Data#with with no args returns self; with args, a new object
Coord = Data.define(:lat, :lng) #: [Float, Float]

here = Coord.new(lat: 1.0, lng: 2.0)
puts here.with.equal?(here).inspect, here.with(lat: 1.0).equal?(here).inspect, (here.with == here).inspect

# frozen? on a plain object, a Struct and a Data
class Plain
end

Pair = Struct.new(:a, :b) #: [Integer, Integer]
Val = Data.define(:v) #: [Integer]

puts Plain.new.frozen?.inspect
puts Pair.new(1, 2).frozen?.inspect
puts Val.new(1).frozen?.inspect

# is_a? on a local used nowhere else
class Animal
end

class Dog < Animal
end

d = Dog.new
puts d.is_a?(Animal).inspect

# constants/const_get/const_defined? see an included module's constants
module Config
  LIMIT = 5
end

class Uses
  include Config

  OWN = 1
end

puts Uses.constants.sort.inspect
puts Uses.const_defined?(:LIMIT).inspect
puts Uses.const_get(:LIMIT).inspect

# nil.class is NilClass, including through a String? param

#: (String?) -> String
def kind(s) = s.class.name

puts kind("a")
puts kind(nil)
puts nil.class

# optional constants at top level and in a module
NAME = "n" #: String?
COUNT = 3 #: Integer?

module Labels
  LABEL = "l" #: String?
end

puts NAME.inspect, COUNT.inspect, Labels::LABEL.inspect
puts NAME.upcase if NAME
puts (COUNT || 0) + 1

# Struct/Data members named like keywords (begin, end, in, out)
Span = Struct.new(:begin, :end) #: [Integer, Integer]
Link = Data.define(:in, :out) #: [String, String]

span = Span.new(1, 5)
puts span.begin.inspect, span.end.inspect, span.inspect, span.to_a.inspect, span.to_h.inspect
span.end = 7
puts (span == Span.new(1, 7)).inspect
l = Link.new(in: "a", out: "b")
puts l.in, l.inspect, l.with(out: "c").inspect

# a member named `other` does not shadow ==(other)
Edge = Struct.new(:from, :other) #: [String, String]
LinkOther = Data.define(:other) #: [Integer]

puts (Edge.new("a", "b") == Edge.new("a", "b")).inspect, (Edge.new("a", "b") == Edge.new("a", "c")).inspect
puts (LinkOther.new(1) == LinkOther.new(1)).inspect, (LinkOther.new(1) == LinkOther.new(2)).inspect
