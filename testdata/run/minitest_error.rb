# rbs_inline: enabled
# An erroring test and an assert_raises that saw the wrong class print the
# frames MRI prints (decision 106): the test's own, up to minitest's.
# args: --seed 1

require "minitest/autorun"

class MinitestErrorTest < Minitest::Test
  #: () -> void
  def helper = raise(ArgumentError, "boom")

  def test_err
    [1].each { helper }
  end

  def test_raises_wrong
    assert_raises(TypeError) { helper }
  end

  def test_ok = assert_equal(2, 1 + 1)
end
