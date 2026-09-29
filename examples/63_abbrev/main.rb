# rbs_inline: enabled
# args: --seed 1

require "abbrev"
require "minitest/autorun"

# Hashes are compared by inspect so key order is checked too.
class AbbrevTest < Minitest::Test
  def test_unambiguous_prefixes_map_to_their_word
    assert_equal '{"ruby" => "ruby", "rub" => "ruby", "rules" => "rules", "rule" => "rules", "rul" => "rules"}',
      Abbrev.abbrev(%w[ruby rules]).inspect
    assert_equal '{"cars" => "cars", "care" => "care", "car" => "car"}', Abbrev.abbrev(%w[car cars care]).inspect
  end

  def test_array_abbrev
    assert_equal '{"summer" => "summer", "summe" => "summer", "summ" => "summer", "sum" => "summer", "su" => "summer", "s" => "summer", ' \
      '"winter" => "winter", "winte" => "winter", "wint" => "winter", "win" => "winter", "wi" => "winter", "w" => "winter"}',
      %w[summer winter].abbrev.inspect
  end

  def test_a_string_or_regexp_pattern_filters_the_results
    assert_equal '{"car" => "car", "ca" => "car"}', Abbrev.abbrev(%w[car box cone crab], "ca").inspect
    assert_equal '{"box" => "box", "bo" => "box", "b" => "box", "crab" => "crab"}', Abbrev.abbrev(%w[car box cone crab], /b/).inspect
  end

  def test_edge_cases
    assert_equal({}, Abbrev.abbrev([]))
    assert_equal({ "x" => "x" }, Abbrev.abbrev(%w[x]))
  end
end
