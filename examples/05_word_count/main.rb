# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class WordCountTest < Minitest::Test
  TEXT = "the cat and the hat and the bat"

  # Most frequent first, ties broken alphabetically by the `[-n, w]` key.
  #: () -> void
  def test_top_three
    counts = TEXT.split.tally
    top = counts.sort_by { |w, n| [-n, w] }.first(3).map { |w, n| "#{w}: #{n}" }
    assert_equal ["the: 3", "and: 2", "bat: 1"], top
  end
end
