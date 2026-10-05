# rbs_inline: enabled

require "minitest/autorun"

# Fiber (decision 140): one goroutine per fiber, handed control over channels, against MRI.
module FiberTests
  class FiberTest < Minitest::Test
    def test_resume_and_yield
      f = Fiber.new do |x|
        y = Fiber.yield x * 2
        y + 1
      end
      assert_equal 10, f.resume(5)
      assert f.alive?
      assert_equal 11, f.resume(10)
      refute f.alive?
    end

    def test_generator
      gen = Fiber.new do
        a, b = 0, 1
        loop do
          Fiber.yield a
          a, b = b, a + b
        end
      end
      assert_equal [0, 1, 1, 2, 3, 5], 6.times.map { gen.resume }
      counter = Fiber.new do
        3.times { |i| Fiber.yield i }
        :done
      end
      assert_equal [0, 1, 2, :done], 4.times.map { counter.resume }
    end

    def test_values
      assert_equal [1, 2], Fiber.new { Fiber.yield 1, 2 }.resume
      assert_nil Fiber.new { Fiber.yield }.resume
      assert_equal "nil", Fiber.new { |v| v.inspect }.resume
      two = Fiber.new do
        got = Fiber.yield
        got
      end
      two.resume
      assert_equal [3, 4], two.resume(3, 4)
    end

    def test_current
      assert Fiber.current.alive?
      assert Fiber.new { Fiber.current.alive? }.resume
      outer = Fiber.current
      refute Fiber.new { Fiber.current == outer }.resume
    end

    def test_errors
      done = Fiber.new { 1 }
      done.resume
      err = assert_raises(FiberError) { done.resume }
      assert_equal "attempt to resume a terminated fiber", err.message
      err = assert_raises(FiberError) { Fiber.yield 1 }
      assert_equal "attempt to yield on a not resumed fiber", err.message
      me = Fiber.new { Fiber.current.resume }
      err = assert_raises(FiberError) { me.resume }
      assert_equal "attempt to resume the current fiber", err.message
      boom = Fiber.new { raise ArgumentError, "boom" }
      err = assert_raises(ArgumentError) { boom.resume }
      assert_equal "boom", err.message
      refute boom.alive?
    end

    def test_nested
      inner = Fiber.new do
        Fiber.yield :inner_a
        :inner_b
      end
      outer = Fiber.new do
        a = inner.resume
        Fiber.yield a
        inner.resume
      end
      assert_equal :inner_a, outer.resume
      assert_equal :inner_b, outer.resume
    end
  end
end
