# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

#: (Integer) -> Integer
def collatz_steps(n)
  steps = 0
  while n != 1
    n = n.even? ? n / 2 : 3 * n + 1
    steps += 1
  end
  steps
end

#: (Integer) -> String
def fizzbuzz(i)
  case i % 15
  when 0 then "FizzBuzz"
  when 3, 6, 9, 12 then "Fizz"
  when 5, 10 then "Buzz"
  else i.to_s
  end
end

class ControlFlowTest < Minitest::Test
  #: () -> void
  def test_while_loop
    assert_equal 111, collatz_steps(27)
  end

  #: () -> void
  def test_case_when_lists
    out = [] #: Array[String]
    1.upto(15) { |i| out << fizzbuzz(i) }
    assert_equal "1 2 Fizz 4 Buzz Fizz 7 8 Fizz Buzz 11 Fizz 13 14 FizzBuzz", out.join(" ")
  end

  # `next` skips odd numbers; `break` stops past 6.
  #: () -> void
  def test_next_and_break
    total = 0
    10.times do |i|
      next if i.odd?
      break if i > 6

      total += i
    end
    assert_equal 12, total
  end

  #: () -> void
  def test_until_modifier
    i = 0
    i += 1 until i * i > 50
    assert_equal 8, i
  end

  #: () -> void
  def test_unless_else
    total = 12
    result = unless total.zero?
               "non-zero"
             else
               "zero"
             end
    assert_equal "non-zero", result
  end

  #: () -> void
  def test_if_elsif_expression
    x = 7
    label = if x > 5 && x < 10
              "mid"
            elsif x >= 10
              "high"
            else
              "low"
            end
    assert_equal "mid", label
  end

  # Integer division and modulo floor toward negative infinity, like Ruby.
  #: () -> void
  def test_integer_arithmetic
    assert_equal [-4, 2, 1024], [-7 / 2, -7 % 3, 2**10]
  end
end
