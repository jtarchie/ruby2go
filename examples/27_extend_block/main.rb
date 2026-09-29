# rbs_inline: enabled
# args: --seed 1
# `extend Enumerable` on a module whose `each` walks its own constants,
# `&block` parameters forwarded into iterators and closures, and user code
# reopening a core class.

require "minitest/autorun"

class String
  #: () -> String
  def camelize = split("_").map(&:capitalize).join
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

  # MRI orders constants by its symbol table; sort for a stable order.
  #: () { (singleton(Base)) -> void } -> void
  def self.each(&block)
    constants.sort.map { |c| const_get(c) }.each(&block)
  end
end

class Registry
  #: () -> void
  def initialize
    @names = [] #: Array[String]
  end

  #: (String) -> self
  def add(name)
    @names << name
    self
  end

  # forwarded into an iterator
  #: () { (String) -> void } -> void
  def each(&block) = @names.each(&block)

  # forwarded into a closure
  #: () { (String) -> String } -> Array[String]
  def transform(&block) = @names.map(&block)

  # called directly
  #: () { (String) -> String } -> String
  def first_transformed(&block) = block.call(@names[0].to_s)
end

class ExtendBlockTest < Minitest::Test
  # Enumerable's methods, driven by the module's own `each`.
  #: () -> void
  def test_extend_enumerable
    assert_equal "Handlers::Users", Handlers.detect { |h| h.matches?("/users/1") }.inspect
    assert_equal ["Handlers::Base", "Handlers::Posts", "Handlers::Users"], Handlers.map { |h| h.name }
    assert_equal 3, Handlers.count
    assert_equal "[Handlers::Posts]", Handlers.select { |h| h.matches?("/posts/9") }.inspect
  end

  #: () -> void
  def test_reopened_core_class
    assert_equal "UserSessions", "user_sessions".camelize
    assert_equal "Posts", "posts".camelize
  end

  #: () -> void
  def test_block_forwarding
    registry = Registry.new.add("a").add("b")
    seen = [] #: Array[String]
    registry.each { |name| seen << name }
    assert_equal ["a", "b"], seen
    assert_equal ["A", "B"], registry.transform { |name| name.upcase }
    assert_equal "aaa", registry.first_transformed { |name| name * 3 }
  end
end
