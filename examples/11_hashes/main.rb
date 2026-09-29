# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class HashesTest < Minitest::Test
  #: () -> Hash[String, Integer]
  def ages
    ages = { "alice" => 30, "bob" => 25 }
    ages["carol"] = 35
    ages
  end

  # Hashes keep insertion order.
  #: () -> void
  def test_insert_and_inspect
    assert_equal 3, ages.size
    assert_equal '{"alice" => 30, "bob" => 25, "carol" => 35}', ages.inspect
    assert_equal ["alice", "bob", "carol"], ages.keys
    assert_equal [30, 25, 35], ages.values
  end

  #: () -> void
  def test_key_and_fetch
    assert_equal true, ages.key?("bob")
    assert_equal false, ages.key?("dave")
    assert_equal 30, ages.fetch("alice")
    assert_equal 0, ages.fetch("dave", 0)
  end

  # Blocks destructure each entry into |key, value|.
  #: () -> void
  def test_each_and_each_pair
    out = [] #: Array[String]
    ages.each { |name, age| out << "#{name}: #{age}" }
    ages.each_pair { |name, age| out << "#{name}=#{age}" }
    assert_equal ["alice: 30", "bob: 25", "carol: 35", "alice=30", "bob=25", "carol=35"], out
  end

  # `delete` returns the removed value, then nil.
  #: () -> void
  def test_delete
    h = ages
    assert_equal 25, h.delete("bob")
    assert_nil h.delete("bob")
    assert_equal '{"alice" => 30, "carol" => 35}', h.inspect
  end

  #: () -> void
  def test_enumerable_on_hash
    h = ages
    h.delete("bob")
    assert_equal ["carol", 35], h.max_by { |_name, age| age }
    assert_equal "alice30,carol35", h.sort_by { |name, _age| name }.map { |name, age| "#{name}#{age}" }.join(",")
    assert_equal '{"carol" => 35}', h.select { |_n, a| a > 30 }.inspect
    assert_equal [["ALICE", 31], ["CAROL", 36]], h.map { |n, a| [n.upcase, a + 1] }
    assert_equal 2, h.count
    assert_equal false, h.empty?
    assert_equal true, {}.empty? #: Hash[String, Integer]
  end

  #: () -> void
  def test_group_by_and_tally
    words = "b a c a b a".split
    assert_equal '{1 => ["b", "a", "c", "a", "b", "a"]}', words.group_by { |w| w.size }.inspect
    assert_equal '{"b" => 2, "a" => 3, "c" => 1}', words.tally.inspect
    assert_equal ["a", "b", "c"], words.uniq.sort
    assert_equal [["a", 3], ["b", 2], ["c", 1]], words.tally.sort_by { |w, n| [-n, w] }
  end

  #: () -> void
  def test_nested_equality
    nested = { "k" => [1, 2] }
    assert_equal '{"k" => [1, 2]}', nested.inspect
    assert_equal true, nested == { "k" => [1, 2] }
  end

  #: () -> void
  def test_each_with_index
    out = [] #: Array[String]
    "b a c a b a".split.each_with_index { |w, i| out << "#{w}#{i}" }
    assert_equal "b0a1c2a3b4a5", out.join
  end
end
