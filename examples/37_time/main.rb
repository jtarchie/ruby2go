# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# A log entry stamped in UTC, so output is the same in every time zone.
class Entry
  attr_reader :at #: Time
  attr_reader :msg #: String

  #: (Time, String) -> void
  def initialize(at, msg)
    @at = at
    @msg = msg
  end

  #: () -> String
  def to_s = "[#{at.strftime("%Y-%m-%d %H:%M:%S")}] #{msg}"
end

#: (Float) -> String
def human(secs)
  mins = (secs / 60).to_i
  "#{mins}m#{(secs - mins * 60).to_i}s"
end

BASE = Time.utc(2024, 2, 29, 23, 59, 30)

#: () -> Array[Entry]
def entries
  [
    Entry.new(BASE, "start"),
    Entry.new(BASE + 45, "leap day rolls over"),
    Entry.new(BASE + 3.5, "fractional"),
    Entry.new(Time.at(1_700_000_000).utc, "epoch 1.7e9"),
  ]
end

class TimeTest < Minitest::Test
  def test_arithmetic_crosses_the_leap_day_and_sorts
    sorted = entries.sort_by(&:at).map(&:to_s)
    assert_equal [
      "[2023-11-14 22:13:20] epoch 1.7e9",
      "[2024-02-29 23:59:30] start",
      "[2024-02-29 23:59:33] fractional",
      "[2024-03-01 00:00:15] leap day rolls over",
    ], sorted
  end

  def test_to_s_and_inspect
    assert_equal "2024-02-29 23:59:30 UTC", BASE.to_s
    assert_equal "2024-02-29 23:59:30 UTC", BASE.inspect
    assert_equal "2024-02-29 23:59:30.25 UTC", (BASE + 0.25).inspect
  end

  def test_iso8601
    assert_equal "2024-02-29T23:59:30Z", BASE.iso8601
    assert_equal "2024-02-29T23:59:30.000Z", BASE.iso8601(3)
  end

  def test_strftime
    assert_equal "Thu 29 Feb 2024, 11:59 PM (UTC) day 060, week 09",
      BASE.strftime("%a %-d %b %Y, %l:%M %p (%Z) day %j, week %V")
    assert_equal "Thursday|FEBRUARY|  Thursday|2/29/24|1709251170",
      BASE.strftime("%A|%^B|%10A|%-m/%-d/%y|%s")
  end

  def test_calendar_fields
    assert_equal [2024, 2, 29], [BASE.year, BASE.month, BASE.day]
    assert_equal 4, BASE.wday
    assert_equal 60, BASE.yday
    assert_equal true, BASE.thursday?
  end

  def test_differences_and_comparison
    later = entries.map(&:at).max || BASE
    assert_equal "2024-03-01 00:00:15 UTC", later.inspect
    assert_equal "0m45s", human(later - BASE)
    assert_equal true, BASE < later
    assert_equal true, BASE == Time.utc(2024, 2, 29, 23, 59, 30)
  end

  def test_epoch
    assert_equal "1970-01-01 00:00:00 UTC", Time.at(0).utc.to_s
    assert_equal 0, Time.at(0).utc.to_i
  end

  def test_invalid_month_raises
    e = assert_raises(ArgumentError) { Time.utc(2024, 13, 1) }
    assert_equal "mon out of range", e.message
  end

  def test_now_moves_forward
    started = Time.now
    assert_equal true, (Time.now - started) >= 0
  end
end
