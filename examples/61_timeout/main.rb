# rbs_inline: enabled
# args: --seed 1
require "timeout"
require "minitest/autorun"

# Timeout.timeout races a block against a wall-clock deadline. Go has no
# thread preemption, so a stuck block cannot actually be interrupted: this
# can only notice the deadline passed and raise from the caller's side,
# leaving the block's own goroutine to keep running in the background.

#: (Float) -> void
def busy_wait(secs)
  started = Time.now
  while Time.now - started < secs
  end
end

class RetryBudgetExceeded < StandardError; end

#: () -> Integer
def slow_computation
  busy_wait(2.0)
  42
end

#: () -> Integer
def fast_computation
  1 + 1
end

class TimeoutTest < Minitest::Test
  def test_a_block_past_its_deadline_raises_timeout_error
    e = assert_raises(Timeout::Error) { Timeout.timeout(0.1) { slow_computation } }
    assert_equal "execution expired", e.message
  end

  def test_a_block_within_its_deadline_returns_its_value
    assert_equal 2, Timeout.timeout(5.0) { fast_computation }
  end

  def test_a_custom_error_class_and_message
    e = assert_raises(RetryBudgetExceeded) do
      Timeout.timeout(0.1, RetryBudgetExceeded, "retry budget exceeded") { slow_computation }
    end
    assert_equal "retry budget exceeded", e.message
  end

  def test_nil_means_no_deadline
    assert_equal 2, Timeout.timeout(nil) { fast_computation }
  end
end
