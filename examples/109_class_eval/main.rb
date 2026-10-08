# rbs_inline: enabled

# class_eval/module_eval in a class body run their block as class-body code,
# so `def` inside defines a method (decision 164).

module Greeting
  module_eval do
    #: () -> String
    def hi = "hi"

    #: (String) -> String
    def greet(name) = "#{hi} #{name}"
  end
end

class Greeter
  include Greeting

  class_eval do
    attr_reader :title #: String

    #: (String) -> void
    def initialize(title)
      @title = title
    end

    #: () -> String
    def announce = "#{greet(title)}!"
  end
end

puts Greeter.new("ruby").announce
puts Greeter.new("go").hi
