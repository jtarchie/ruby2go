# rbs_inline: enabled
# args: --seed 1
# Thread.new / join. An exception in a thread re-raises on join.

require "minitest/autorun"

class ThreadsTest < Minitest::Test
  #: () -> void
  def test_join_waits_for_the_block
    results = []
    worker = Thread.new { results << 42 }
    worker.join
    assert_equal [42], results
  end

  #: () -> void
  def test_exception_reraises_on_join
    failing = Thread.new { raise ArgumentError, "boom" }
    e = assert_raises(ArgumentError) { failing.join }
    assert_equal "boom", e.message
  end
end
