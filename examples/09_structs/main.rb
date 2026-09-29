# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class Animal
  attr_reader :name #: String
  attr_accessor :legs #: Integer

  #: (String, Integer) -> void
  def initialize(name, legs)
    @name = name
    @legs = legs
    @sound = "..." #: String
  end

  #: () -> String
  def speak = "#{name} says #{@sound}"

  #: () -> String
  def to_s = "#{name} (#{legs} legs)"
end

class Dog < Animal
  #: (String) -> void
  def initialize(name)
    super(name, 4)
    @sound = "woof"
    @tricks = [] #: Array[String]
  end

  #: (String) -> self
  def learn(trick)
    @tricks << trick
    self
  end

  #: () -> Integer
  def trick_count = @tricks.size

  def speak = super + "!"
end

class Bird < Animal
  #: (String) -> void
  def initialize(name) = super(name, 2)

  def speak = "#{name} tweets"
end

class StructsTest < Minitest::Test
  #: () -> Array[Animal]
  def zoo = [Dog.new("Rex"), Bird.new("Tweety"), Animal.new("Blob", 0)]

  # Each subclass overrides `speak`; Dog's calls `super`.
  #: () -> void
  def test_overrides_and_super
    assert_equal ["Rex says woof!", "Tweety tweets", "Blob says ..."], zoo.map(&:speak)
  end

  # `to_s` is inherited from Animal and used by interpolation.
  #: () -> void
  def test_to_s
    assert_equal ["Rex (4 legs)", "Tweety (2 legs)", "Blob (0 legs)"], zoo.map { |a| "#{a}" }
  end

  # `learn` returns self, so calls chain.
  #: () -> void
  def test_chaining_self
    rex = Dog.new("Rex")
    rex.learn("sit").learn("roll")
    assert_equal 2, rex.trick_count
  end

  #: () -> void
  def test_attr_accessor
    rex = Dog.new("Rex")
    rex.legs = 3
    assert_equal "Rex (3 legs)", rex.to_s
  end

  # `==` defaults to identity.
  #: () -> void
  def test_identity_equality
    rex = Dog.new("Rex")
    assert_equal [true, false, true], [rex == rex, rex == Dog.new("Rex"), rex.equal?(rex)]
  end

  #: () -> void
  def test_attr_reader_across_subclasses
    assert_equal [4, 2, 0], zoo.map { |a| a.legs }
  end
end
