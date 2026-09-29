# rbs_inline: enabled
# args: --seed 1
# `||=` on locals and instance variables (memoization).

require "minitest/autorun"

class Cache
  #: () -> void
  def initialize
    @calls = 0
  end

  #: () -> Integer
  def calls = @calls

  #: () -> String
  def value
    @value ||= compute
  end

  private

  #: () -> String
  def compute
    @calls += 1
    "computed"
  end
end

class OrAssignTest < Minitest::Test
  # The second call returns the memoized value without computing again.
  #: () -> void
  def test_memoized_ivar
    cache = Cache.new
    assert_equal ["computed", "computed"], [cache.value, cache.value]
    assert_equal 1, cache.calls
  end

  #: () -> void
  def test_or_assign_locals
    name = nil
    name ||= "default"
    assert_equal "default", name
    count = nil
    count ||= 1
    count ||= 2
    assert_equal 1, count
  end
end
