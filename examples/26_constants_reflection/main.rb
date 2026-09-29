# rbs_inline: enabled
# args: --seed 1
# Class objects for every class and module; constants, const_get,
# const_defined?, Object.const_get with paths. MRI orders `constants` by its
# symbol table, so this example sorts before comparing.

require "minitest/autorun"

module Formats
  VERSION = "2.0" #: String

  class Base
    #: (String) -> bool
    def self.matches?(path) = false
  end

  class HTML < Base
    def self.matches?(path) = path.end_with?(".html")
  end

  class JSON < Base
    def self.matches?(path) = path.end_with?(".json")
  end
end

module Handlers
  class Base
    #: () -> String
    def self.kind = "base"
  end

  class Index < Base
    def self.kind = "index"
  end

  class Show < Base
    def self.kind = "show"
  end
end

# Every constant of Handlers is a class under Base, so const_get is typed
# singleton(Handlers::Base) and class methods can be called on the result.
#: (String) -> String
def kind_for(name) = Handlers.const_get(name).kind

class ConstantsReflectionTest < Minitest::Test
  # Modules and classes are objects with a name and a class.
  #: () -> void
  def test_class_objects
    assert_equal "Formats", Formats.name
    assert_equal "Formats::HTML", Formats::HTML.name
    assert_equal "Module", Formats.class.to_s
    assert_equal "Class", Formats::HTML.class.to_s
    assert_equal "String", String.to_s
    assert_equal "Class", String.class.to_s
    assert_equal "Integer", Integer.name
    assert_equal "Integer", 42.class.to_s
    assert_equal true, "s".class == String
  end

  #: () -> void
  def test_constants_and_const_defined
    assert_equal [:Base, :HTML, :JSON, :VERSION], Formats.constants.sort
    assert_equal [:Base, :Index, :Show], Handlers.constants.sort
    assert_equal true, Formats.const_defined?(:HTML)
    assert_equal false, Formats.const_defined?("XML")
  end

  # const_get's result is typed, so class methods can be called on it.
  #: () -> void
  def test_const_get
    assert_equal ["index", "show", "base"], [kind_for("Index"), kind_for("Show"), Handlers.const_get(:Base).kind]
    assert_equal "2.0", Formats.const_get(:VERSION)
    assert_equal "Formats::JSON", Object.const_get("Formats::JSON").to_s
    assert_equal "Handlers", Object.const_get(:Handlers).to_s
    assert_equal true, Object.const_get("Formats::JSON") == Formats::JSON
  end

  #: () -> void
  def test_filter_constants_by_class_method
    matching = Handlers.constants.sort.map { |c| Handlers.const_get(c) }.select { |k| k.kind.size > 4 }
    assert_equal "[Handlers::Index]", matching.inspect
  end

  # A path stops at the first missing constant.
  #: () -> void
  def test_missing_constants_raise_name_error
    e = assert_raises(NameError) { Formats.const_get(:XML) }
    assert_equal "uninitialized constant Formats::XML", e.message
    e = assert_raises(NameError) { Object.const_get("Formats::Nope::Deeper") }
    assert_equal "uninitialized constant Formats::Nope", e.message
  end
end
