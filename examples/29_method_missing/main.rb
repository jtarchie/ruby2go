# rbs_inline: enabled
# args: --seed 1
# method_missing and respond_to? on statically typed receivers: an unknown
# method compiles to a method_missing call, typed by its signature.

require "minitest/autorun"

class NullObject
  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = nil

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = true
end

class Settings
  #: () -> void
  def initialize
    @values = { "color" => "blue", "size" => "L" } #: Hash[String, String]
  end

  #: () -> String
  def to_s = "settings"

  #: (Symbol, *untyped) -> String
  def method_missing(name, *args)
    value = @values[name.to_s]
    return value if value
    "(no #{name}, #{args.size} args)"
  end

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = @values.key?(name.to_s)
end

class Plain
  #: () -> String
  def hello = "hi"
end

class MethodMissingTest < Minitest::Test
  # Unknown methods reach method_missing with their name and arguments.
  #: () -> void
  def test_method_missing
    settings = Settings.new
    assert_equal "blue", settings.color
    assert_equal "L", settings.size
    assert_equal "(no weight, 0 args)", settings.weight
    assert_equal "(no weight, 2 args)", settings.weight(1, "two")
  end

  # respond_to? consults respond_to_missing?; private methods don't count.
  #: () -> void
  def test_respond_to_missing
    settings = Settings.new
    assert_equal true, settings.respond_to?(:color)
    assert_equal false, settings.respond_to?(:weight)
    assert_equal true, settings.respond_to?(:to_s)
    assert_equal false, settings.respond_to?(:initialize)
  end

  #: () -> void
  def test_null_object
    null = NullObject.new
    assert_nil null.anything
    assert_nil null.deeply(1, "two")
    assert_equal true, null.respond_to?(:x)
  end

  #: () -> void
  def test_respond_to_without_method_missing
    assert_equal true, Plain.new.respond_to?(:hello)
    assert_equal false, Plain.new.respond_to?(:bye)
    assert_equal true, 5.respond_to?(:even?)
  end
end
