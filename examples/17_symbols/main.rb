# rbs_inline: enabled
# args: --seed 1
# Symbols, and braceless hash arguments (`f(a: 1)` passes a Hash).

require "minitest/autorun"

#: (String, ?Hash[Symbol, String]) -> String
def tag(name, attrs = {})
  rendered = attrs.map { |key, value| " #{key}=\"#{value}\"" }.join
  "<#{name}#{rendered}>"
end

#: (Hash[String, Integer]) -> Integer
def total(counts) = counts.values.reduce(0) { |sum, n| sum + n }

class SymbolsTest < Minitest::Test
  #: () -> void
  def test_symbol_keys
    config = { port: 8080, host: "localhost" }
    assert_equal 8080, config[:port]
    assert_equal "localhost", config[:host]
    assert_equal [:port, :host], config.keys
  end

  #: () -> void
  def test_symbol_basics
    assert_equal ":abc", :abc.inspect
    assert_equal "abc", :abc.to_s
    assert_equal true, :a == :a
    assert_equal false, :a == :b
  end

  # Trailing `key: value` arguments arrive as one Hash.
  #: () -> void
  def test_braceless_hash_arguments
    assert_equal "<br>", tag("br")
    assert_equal '<a href="/x" id="y">', tag("a", href: "/x", id: "y")
    assert_equal 3, total("a" => 1, "b" => 2)
  end

  # Symbol keys that aren't plain identifiers inspect quoted.
  #: () -> void
  def test_hash_inspect_key_styles
    h = { port: 1, "a b": 2, :+ => 3, ok?: 4, "s" => 5, "set=": 6 }
    assert_equal '{port: 1, "a b": 2, "+": 3, ok?: 4, "s" => 5, "set=": 6}', h.inspect
  end
end
