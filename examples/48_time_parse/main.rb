# rbs_inline: enabled
# args: --seed 1

require "time"
require "benchmark"
require "minitest/autorun"

# Log lines stamped in assorted formats, normalised to UTC and bucketed by hour.
class LogLine
  attr_reader :at #: Time
  attr_reader :msg #: String

  #: (Time, String) -> void
  def initialize(at, msg)
    @at = at
    @msg = msg
  end

  #: (String) -> LogLine
  def self.parse(line)
    stamp, msg = line.split(" | ")
    new(Time.parse(stamp || ""), msg || "")
  end
end

LINES = [
  "2024-03-05T12:34:56Z | boot",
  "2024-03-05 13:05:00 +0200 | login",
  "2024-03-05T09:15:30.250-05:00 | fetch",
  "Tue, 05 Mar 2024 14:00:00 GMT | cron",
] #: Array[String]

class LogLineTest < Minitest::Test
  def test_parse_normalises_every_format_to_utc
    stamped = LINES.map { |l| LogLine.parse(l) }.map { |l| "#{l.at.utc.iso8601(3)} #{l.msg}" }
    assert_equal ["2024-03-05T12:34:56.000Z boot",
                  "2024-03-05T11:05:00.000Z login",
                  "2024-03-05T14:15:30.250Z fetch",
                  "2024-03-05T14:00:00.000Z cron"], stamped
  end

  def test_bucket_by_utc_hour
    by_hour = LINES.map { |l| LogLine.parse(l) }.group_by { |l| l.at.utc.hour }
    buckets = by_hour.keys.sort.map { |h| "#{h}: #{by_hour.fetch(h).map(&:msg).join(", ")}" }
    assert_equal ["11: login", "12: boot", "14: fetch, cron"], buckets
  end
end

class TimeParseTest < Minitest::Test
  # A numeric offset keeps a fixed zone; Z/UTC/GMT give UTC mode.
  def test_iso8601_and_xmlschema
    t = Time.iso8601("2024-03-05T12:34:56+09:00")
    assert_equal "2024-03-05 12:34:56 +0900", t.to_s
    assert_equal 32_400, t.utc_offset
    assert_equal "2024-03-05T12:34:56+09:00", t.iso8601
    assert_equal false, t.utc?
    u = Time.xmlschema("1999-12-31T23:59:59.5Z")
    assert_equal "1999-12-31 23:59:59.5 UTC", u.inspect
    assert_equal true, u.utc?
    assert_equal 500_000, u.usec
  end

  def test_httpdate_and_rfc2822_formatting
    noon = Time.utc(2024, 3, 5, 12, 0, 0)
    assert_equal "Tue, 05 Mar 2024 12:00:00 GMT", noon.httpdate
    assert_equal "Tue, 05 Mar 2024 12:00:00 -0000", noon.rfc2822
    tokyo = Time.iso8601("2024-03-05T12:34:56+09:00")
    assert_equal "Tue, 05 Mar 2024 12:34:56 +0900", tokyo.rfc2822
    assert_equal "Tue, 05 Mar 2024 03:34:56 GMT", tokyo.httpdate
  end

  def test_httpdate_and_rfc2822_parsing
    h = Time.httpdate("Sun, 06 Nov 1994 08:49:37 GMT")
    assert_equal "1994-11-06 08:49:37 UTC", h.to_s
    assert_equal true, h.utc?
    assert_equal 0, h.wday
    r = Time.rfc2822("Sun, 6 Nov 1994 08:49:37 -0800")
    assert_equal "1994-11-06 08:49:37 -0800", r.to_s
    assert_equal(-28_800, r.utc_offset)
  end

  def test_strptime
    s = Time.strptime("05/03/2024 07:08:09 +0100", "%d/%m/%Y %H:%M:%S %z")
    assert_equal "2024-03-05 07:08:09 +0100", s.to_s
    assert_equal 8, s.min
    # %j is the day of the year: day 61 of 2024 is March 1st.
    s2 = Time.strptime("2024-061 23:59", "%Y-%j %H:%M")
    assert_equal [3, 1, 23], [s2.month, s2.day, s2.hour]
    s3 = Time.strptime("March 7 2024 3:04 PM +0000", "%B %d %Y %I:%M %p %z")
    assert_equal 15, s3.hour
    assert_equal 0, s3.utc_offset
  end

  def test_parse_with_a_utc_zone
    t = Time.parse("2024-03-05 01:02:03 UTC")
    assert_equal true, t.utc?
    assert_equal "2024-03-05 01:02:03 UTC", t.to_s
  end

  def test_bad_input_raises
    e = assert_raises(ArgumentError) { Time.iso8601("yesterday") }
    assert_equal "invalid xmlschema format: \"yesterday\"", e.message
    e = assert_raises(ArgumentError) { Time.strptime("nope", "%Y") }
    assert_equal "invalid date or strptime format - 'nope' '%Y'", e.message
    e = assert_raises(ArgumentError) { Time.parse("") }
    assert_equal "no time information in \"\"", e.message
  end

  # Elapsed time varies, so only its type and sign are checked.
  def test_benchmark_realtime
    elapsed = Benchmark.realtime { (1..1000).sum }
    assert_kind_of Float, elapsed
    assert_equal true, elapsed >= 0
  end
end
