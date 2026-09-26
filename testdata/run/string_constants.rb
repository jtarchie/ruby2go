# frozen_string_literal: true

# rbs_inline: enabled

# Strings and Symbols held in constants, ivars (attr_accessor, ||=) and default parameters.
GREETING = "hi"
MODE = :fast
NAMES = %w[ann bob]
SEP = ", " #: String

class Person
  attr_accessor :name #: String
  attr_reader :nick #: String?

  #: (String) -> void
  def initialize(name)
    @name = name
    @nick = nil
  end

  #: () -> String
  def nick_or_name
    @nick ||= name.downcase
    @nick || ""
  end

  #: (?String, ?String) -> String
  def greet(greeting = GREETING, punct = "!") = "#{greeting}, #{name}#{punct}"
end

puts GREETING.upcase, GREETING.frozen?.inspect, "#{GREETING} #{MODE}", MODE.inspect, NAMES.join(SEP), NAMES.map(&:capitalize).inspect
said = "h" + "i"
case said
when GREETING then puts "matched const"
else puts "no"
end
m = :fast
case m
when MODE then puts "mode const"
end
p1 = Person.new("Ann")
puts p1.greet, p1.greet("yo"), p1.greet("hey", "?"), p1.nick_or_name, p1.nick.inspect
p1.name = p1.name + " Lee"
puts p1.name, p1.greet
puts GREETING.equal?(GREETING).inspect
