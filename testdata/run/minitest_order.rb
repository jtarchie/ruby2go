# Run order with several test classes (a nested one, a grandchild) is MRI's: -v names each test in order.
# args: --seed 7 -v
require "minitest/autorun"

class ATest < Minitest::Test
  def test_a1 = assert(true)
  def test_a2 = assert(true)
end

module Outer
  class BTest < Minitest::Test
    def test_b = assert(true)
  end
end

class CTest < ATest
  def test_c = assert(true)
end

class DTest < Minitest::Test
  def test_d1 = assert(true)
  def test_d2 = assert(true)
  def test_d3 = assert(true)
end
