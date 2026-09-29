# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# A seeded dice simulation: MRI's Mersenne Twister, so every run draws the same.
class Dice
  #: (Integer, Integer) -> void
  def initialize(sides, seed)
    @sides = sides
    @rng = Random.new(seed)
  end

  #: () -> Integer
  def roll = @rng.rand(1..@sides)

  #: (Integer) -> Array[Integer]
  def histogram(n)
    counts = Array.new(@sides, 0)
    n.times do
      i = roll - 1
      counts[i] = (counts[i] || 0) + 1
    end
    counts
  end
end

DECK = %w[A K Q J 10 9 8 7] #: Array[String]

class RandomTest < Minitest::Test
  def test_seeded_dice_roll_the_same_as_mri
    d = Dice.new(6, 2024)
    assert_equal [1, 3, 1, 1, 4, 5, 2, 2, 5, 4], (1..10).map { d.roll }
    assert_equal [102, 95, 96, 95, 113, 99], Dice.new(6, 7).histogram(600)
  end

  def test_random_draws_integers_floats_and_big_numbers
    rng = Random.new(42)
    assert_equal 51, rng.rand(100)
    assert_equal 0.950714, rng.rand.round(6)
    assert_equal 1.829985, rng.rand(2.5).round(6)
    assert_equal 810_017_303_572, rng.rand(10**12)
    assert_equal 42, rng.seed
  end

  def test_shuffle_and_sample_take_a_random_generator
    assert_equal "7 Q K 8 A 10 J 9", DECK.shuffle(random: Random.new(1)).join(" ")
    assert_equal "Q", DECK.sample(random: Random.new(3))
    assert_equal %w[Q A J], DECK.sample(3, random: Random.new(3))
    hand = DECK.dup
    hand.shuffle!(random: Random.new(5))
    assert_equal %w[7 Q 10 K], hand.first(4)
  end

  # Kernel#rand and srand share one generator.
  def test_srand_replays_kernel_rand
    srand(99)
    first = [rand(1000), rand(1000)]
    srand(99)
    assert_equal first, [rand(1000), rand(1000)]
    assert_equal true, rand.between?(0.0, 1.0)
    assert_equal true, rand(3..4).between?(3, 4)
  end

  def test_rand_zero_raises
    e = assert_raises(ArgumentError) { Random.new(42).rand(0) }
    assert_equal "invalid argument - 0", e.message
  end
end
