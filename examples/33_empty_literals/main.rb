# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# Blocks on untyped receivers are unsupported, so these only compile once each `[]`/`{}` is typed.

#: (Array[String]) -> Hash[String, Integer]
def tally_by_first(ws)
  out = {}
  ws.each { |w| out[w[0] || ""] = (out[w[0] || ""] || 0) + 1 }
  out
end

#: (Array[untyped]) -> void
def add_marker(xs)
  xs << "marker"
end

class EmptyLiteralsTest < Minitest::Test
  WORDS = %w[pear fig apple fig]

  #: () -> void
  def test_array_typed_by_first_push
    shout = []
    WORDS.each { |w| shout << w.upcase }
    assert_equal ["APPLE", "FIG", "FIG", "PEAR"], shout.sort

    # Keys type only after `shout` does: `w` is untyped until then.
    lengths = {}
    shout.each { |w| lengths[w] = w.size }
    assert_equal '{"PEAR" => 4, "FIG" => 3, "APPLE" => 5}', lengths.inspect
    assert_equal "PEAR=4,FIG=3,APPLE=5", lengths.map { |w, n| "#{w}=#{n}" }.join(",")
  end

  #: () -> void
  def test_array_of_tuples
    ranked = []
    WORDS.each_with_index { |w, i| ranked << [w, i] }
    assert_equal ["fig", "apple", "fig", "pear"], ranked.sort_by { |w, i| [-i, w] }.map { |w, _| w }
  end

  #: () -> void
  def test_array_of_optionals
    found = []
    WORDS.each { |w| found.push(w.index("p")) }
    assert_equal [0, 1], found.compact

    # A gap before the index reads nil, so the elements are Integer?.
    slots = []
    slots[2] = 7
    assert_equal [nil, nil, 7], slots
  end

  #: () -> void
  def test_hash_typed_by_method_return
    assert_equal '{"p" => 1, "f" => 2, "a" => 1}', tally_by_first(WORDS).inspect
  end

  #: () -> void
  def test_mixed_pushes
    bag = []
    bag << 1
    bag << "two"
    assert_equal [1, "two"], bag
  end

  # Typed, it would be copied into Array[untyped] and lose the mutation: stays untyped.
  #: () -> void
  def test_untyped_array_shared_with_callee
    shared = []
    shared << 1
    add_marker(shared)
    assert_equal [1, "marker"], shared
  end
end
