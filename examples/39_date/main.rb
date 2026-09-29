# rbs_inline: enabled
# args: --seed 1

require "date"
require "minitest/autorun"

# A monthly billing schedule: month arithmetic clamps to the month's end.
class Subscription
  attr_reader :start #: Date
  attr_reader :plan #: String

  #: (Date, String) -> void
  def initialize(start, plan)
    @start = start
    @plan = plan
  end

  #: (Integer) -> Array[Date]
  def invoices(n) = (0...n).map { |i| start >> i }

  #: (Date) -> Integer
  def days_active(on) = (on - start).to_i
end

class SubscriptionTest < Minitest::Test
  def setup
    @sub = Subscription.new(Date.new(2024, 1, 31), "pro")
  end

  def test_month_steps_clamp_to_the_end_of_the_month
    assert_equal "2024-01-31 2024-02-29 2024-03-31 2024-04-30", @sub.invoices(4).map(&:to_s).join(" ")
  end

  def test_date_difference_counts_days
    assert_equal 30, @sub.days_active(Date.parse("2024-03-01"))
  end
end

class DateTest < Minitest::Test
  def setup
    @d = Date.parse("March 5, 2024")
  end

  def test_inspect_shows_the_julian_day
    assert_equal "#<Date: 2024-03-05 ((2460375j,0s,0n),+0s,2299161j)>", @d.inspect
  end

  def test_calendar_fields
    assert_equal "2024-03-05 is a Tuesday, day 65, ISO week 10",
      "#{@d} is a #{Date::DAYNAMES[@d.wday]}, day #{@d.yday}, ISO week #{@d.cweek}"
    assert_equal "Tue 5 Mar 2024", @d.strftime("%a %-d %b %Y")
    assert_equal true, @d.leap?
  end

  def test_day_and_month_arithmetic
    assert_equal "2024-04-04", (@d + 30).to_s
    assert_equal "2023-03-05", (@d << 12).to_s
  end

  def test_strptime_and_negative_constructor_args
    assert_equal "2023-12-25", Date.strptime("12/25/2023", "%m/%d/%Y").to_s
    assert_equal "2024-12-31", Date.new(2024, -1, -1).to_s
  end

  def test_date_ranges_upto_and_step
    weekend = (Date.new(2024, 3, 1)..Date.new(2024, 3, 14)).select { |x| x.saturday? || x.sunday? }
    assert_equal [2, 3, 9, 10], weekend.map(&:day)

    days = [] #: Array[String]
    Date.new(2024, 2, 27).upto(Date.new(2024, 3, 1)) { |x| days << x.strftime("%m/%d") }
    assert_equal ["02/27", "02/28", "02/29", "03/01"], days

    by_month = Hash.new #: Hash[Integer, Integer]
    Date.new(2024, 1, 1).step(Date.new(2024, 12, 31), 7) do |x|
      by_month[x.month] = (by_month[x.month] || 0) + 1
    end
    assert_equal "{1 => 5, 2 => 4, 3 => 4, 4 => 5, 5 => 4, 6 => 4, 7 => 5, 8 => 4, 9 => 5, 10 => 4, 11 => 4, 12 => 5}",
      by_month.inspect
  end

  def test_comparison_and_validity
    assert_equal "2024-05-01", [Date.new(2024, 5, 1), Date.new(2023, 5, 1)].max.to_s
    assert_equal false, Date.valid_date?(2023, 2, 29)
    assert_equal true, Date.leap?(2000)
    assert_equal false, Date.leap?(1900)
  end

  def test_invalid_date_raises
    e = assert_raises(Date::Error) { Date.new(2023, 2, 29) }
    assert_equal "invalid date", e.message
  end
end
