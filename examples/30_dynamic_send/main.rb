# rbs_inline: enabled
# args: --seed 1
# Calls on untyped values dispatch at run time: to the method if the
# receiver's class has one, else to method_missing, else NoMethodError.
# send/public_send take literal or computed names; respond_to? works too.

require "minitest/autorun"

class Greeter
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: (?String) -> String
  def greet(greeting = "hello") = "#{greeting}, #{name}"

  #: (*String) -> String
  def shout(*words) = words.map(&:upcase).join(" ")

  #: () -> Integer?
  def lucky = nil
end

class Ghost
  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = "ghost #{name}(#{args.size})"

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("g")
end

class DynamicSendTest < Minitest::Test
  #: () -> Array[untyped]
  def things = [Greeter.new("ada"), Ghost.new]

  # One call site, two receivers: a real method, then method_missing.
  #: () -> void
  def test_untyped_dispatch
    got = things.map do |thing|
      [thing.greet, thing.greet("hi"), thing.shout("a", "b"), thing.respond_to?(:greet), thing.respond_to?(:zap)]
    end
    assert_equal [
      ["hello, ada", "hi, ada", "A B", true, false],
      ["ghost greet(0)", "ghost greet(1)", "ghost shout(2)", true, false],
    ], got
  end

  #: () -> void
  def test_send_and_public_send
    greeter = things.fetch(0)
    assert_equal "yo, ada", greeter.send(:greet, "yo")
    assert_equal "ada", greeter.public_send(:name)
    assert_nil greeter.lucky
    # Computed method names.
    assert_equal ["hello, ada", "", "ada"], ["greet", "shout", "name"].map { |m| greeter.send(m) }
    assert_equal [true, false], ["name", "fly"].map { |m| greeter.respond_to?(m) }
  end

  #: () -> void
  def test_class_from_string
    class_name = "Greeter"
    klass = Object.const_get(class_name)
    assert_equal "hey, bob", klass.new("bob").greet("hey")
    assert_equal "Greeter", klass.name
  end

  #: () -> void
  def test_dynamic_errors
    greeter = things.fetch(0)
    e = assert_raises(NoMethodError) { greeter.fly }
    assert_equal "undefined method 'fly' for an instance of Greeter", e.message
    e = assert_raises(ArgumentError) { greeter.greet("a", "b", "c") }
    assert_equal "wrong number of arguments (given 3, expected 0..1)", e.message
    nothing = nil #: untyped
    e = assert_raises(NoMethodError) { nothing.greet }
    assert_equal "undefined method 'greet' for nil", e.message
  end
end
