# rbs_inline: enabled

require "minitest/autorun"
require "date"

# DateTime (decision 133); expected values are MRI's.
module DateTests
  class DateTimeTest < Minitest::Test
    def dt = DateTime.new(2024, 2, 29, 13, 45, 30, "+09:00")

    def test_readers
      d = dt
      assert_equal "2024-02-29T13:45:30+09:00", d.to_s
      assert_equal "#<DateTime: 2024-02-29T13:45:30+09:00 ((2460370j,17130s,0n),+32400s,2299161j)>", d.inspect
      assert_equal [13, 45, 45, 30, 30], [d.hour, d.minute, d.min, d.second, d.sec]
      assert_equal "+09:00", d.zone
      assert_equal Rational(3, 8), d.offset
      assert_equal Rational(1651, 2880), d.day_fraction
      assert_equal Rational(0, 1), d.sec_fraction
      assert_equal [2024, 2, 29, 4, 60, 2460370, 60369], [d.year, d.month, d.day, d.wday, d.yday, d.jd, d.mjd]
      assert_equal [4, 9, 2024, true, true], [d.cwday, d.cweek, d.cwyear, d.leap?, d.thursday?]
      assert_equal true, d.is_a?(Date)
      assert_equal "DateTime", d.class.to_s
    end

    def test_new_forms
      assert_equal "-4712-01-01T00:00:00+00:00", DateTime.new.to_s
      assert_equal "2024-01-01T00:00:00+00:00", DateTime.civil(2024).to_s
      assert_equal "2024-01-01T23:59:59+00:00", DateTime.new(2024, 1, 1, -1, -1, -1).to_s
      assert_equal "2024-01-02T00:00:00+00:00", DateTime.new(2024, 1, 1, 24).to_s
      assert_equal "#<DateTime: 2024-01-01T10:00:01+01:00 ((2460311j,32401s,500000000n),+3600s,2299161j)>", DateTime.new(2024, 1, 1, 10, 0, Rational(3, 2), Rational(1, 24)).inspect
      assert_equal "#<DateTime: 2024-01-01T10:00:01-05:30 ((2460311j,55801s,250000000n),-19800s,2299161j)>", DateTime.new(2024, 1, 1, 10, 0, 1.25, "-05:30").inspect
      assert_equal "2024-02-29T00:00:00+00:00", DateTime.jd(2460370).to_s
      assert_equal "2024-02-29T12:30:00+00:00", DateTime.jd(2460370, 12, 30).to_s
      assert_raises(NoMethodError) { DateTime.today }
      [[2024, 1, 1, 25, 0, 0], [2024, 1, 1, 0, 0, 60], [2023, 2, 29, 0, 0, 0]].each do |a|
        assert_raises(Date::Error) { DateTime.new(a.fetch(0), a.fetch(1), a.fetch(2), a.fetch(3), a.fetch(4), a.fetch(5)) }
      end
    end

    def test_offsets
      zones = ["-00:00", "Z", "UTC", "EST", "jst", "GMT+3", "+0930", "+9", "+25:00", "bogus"]
      got = zones.map { |z| DateTime.new(2024, 1, 1, 0, 0, 0, z).zone }
      assert_equal ["+00:00", "+00:00", "+00:00", "-05:00", "+09:00", "+03:00", "+09:30", "+09:00", "+00:00", "+00:00"], got
      assert_equal "+24:00", DateTime.new(2024, 1, 1, 0, 0, 0, 1).zone
      assert_equal "+12:00", DateTime.new(2024, 1, 1, 0, 0, 0, 0.5).zone
      assert_equal "#<DateTime: 2024-01-01T00:00:00+03:25 ((2460310j,74057s,0n),+12343s,2299161j)>", DateTime.new(2024, 1, 1, 0, 0, 0, Rational(1, 7)).inspect
      d = dt
      assert_equal "2024-02-29T09:45:30+05:00", d.new_offset("+05:00").to_s
      assert_equal "2024-02-29T04:45:30+00:00", d.new_offset.to_s
      assert_equal "2024-02-28T23:45:30-05:00", d.new_offset(Rational(-5, 24)).to_s
      assert_equal true, d == d.new_offset(0)
    end

    def test_arithmetic
      d = dt
      assert_equal "2024-03-01T13:45:30+09:00", (d + 1).to_s
      assert_equal "2024-02-27T13:45:30+09:00", d.prev_day(2).to_s
      assert_equal "2024-03-01T13:45:30+09:00", d.next_day.to_s
      assert_equal "2024-02-29T07:45:30+09:00", (d - 0.25).to_s
      assert_equal "2024-02-29T21:45:30+09:00", (d + Rational(1, 3)).to_s
      assert_equal "2024-02-29T01:45:30+09:00", (d - Rational(1, 2)).to_s
      assert_equal "2024-03-02T01:45:30+09:00", (d + 1.5).to_s
      assert_equal "DateTime", (d + 1).class.to_s
      assert_equal Rational(2309, 2880), DateTime.new(2024, 3, 1) - d
      assert_equal 0.8017, (DateTime.new(2024, 3, 1) - d).to_f.round(4)
      assert_equal Rational(1, 2), Date.new(2024, 3, 1) - DateTime.new(2024, 2, 29, 12)
    end

    def test_month_arithmetic
      d = dt
      assert_equal "2024-03-29T13:45:30+09:00", (d >> 1).to_s
      assert_equal "2024-01-29T13:45:30+09:00", (d << 1).to_s
      assert_equal "2025-02-28T13:45:30+09:00", d.next_month(12).to_s
      assert_equal "2025-02-28T13:45:30+09:00", d.next_year.to_s
      assert_equal "2023-02-28T13:45:30+09:00", d.prev_year.to_s
      assert_equal "2024-02-29T12:00:00+00:00", (DateTime.new(2024, 1, 31, 12) >> 1).to_s
    end

    def test_comparison
      d = dt
      assert_equal true, d == DateTime.new(2024, 2, 29, 4, 45, 30)
      assert_equal(-1, d <=> DateTime.new(2024, 2, 29, 4, 45, 31))
      assert_equal true, Date.new(2024, 2, 29) == DateTime.new(2024, 2, 29)
      assert_equal(-1, Date.new(2024, 2, 29) <=> d)
      assert_equal true, d.eql?(DateTime.new(2024, 2, 29, 4, 45, 30))
      assert_equal true, d.hash == DateTime.new(2024, 2, 29, 4, 45, 30).hash
      assert_equal true, Date.new(2024, 1, 1).eql?(DateTime.new(2024, 1, 1))
      assert_equal true, DateTime.new(2024, 1, 1, 12) === Date.new(2024, 1, 1)
      h = { DateTime.new(2024, 1, 1) => 1 } #: Hash[DateTime, Integer]
      assert_equal 1, h[DateTime.new(2024, 1, 1)]
      sorted = [DateTime.new(2024, 1, 2), DateTime.new(2024, 1, 1)].sort
      assert_equal ["2024-01-01T00:00:00+00:00", "2024-01-02T00:00:00+00:00"], sorted.map(&:to_s)
      assert_equal "2024-01-02T00:00:00+00:00", [DateTime.new(2024, 1, 2), DateTime.new(2024, 1, 1)].max.to_s
      assert_equal ["2024-01-01T00:00:00+00:00", "2024-01-02T00:00:00+00:00", "2024-01-03T00:00:00+00:00"], (DateTime.new(2024, 1, 1)..DateTime.new(2024, 1, 3)).map(&:to_s)
    end

    def test_formatters
      d = dt
      assert_equal "2024-02-29T13:45:30+09:00", d.iso8601
      assert_equal "2024-02-29T13:45:30.000+09:00", d.iso8601(3)
      assert_equal "2024-02-29T13:45:30+09:00", d.xmlschema
      assert_equal "2024-02-29T13:45:30.0000+09:00", d.xmlschema(4)
      assert_equal "2024-02-29T13:45:30.0+09:00", d.rfc3339(1)
      assert_equal "R06.02.29T13:45:30+09:00", d.jisx0301
      assert_equal "R06.02.29T13:45:30.00+09:00", d.jisx0301(2)
      assert_equal "Thu, 29 Feb 2024 04:45:30 GMT", d.httpdate
      assert_equal "Thu, 29 Feb 2024 13:45:30 +0900", d.rfc2822
      assert_equal "2024-01-01T00:00:00.50+00:00", DateTime.new(2024, 1, 1, 0, 0, 0.5).iso8601(2)
      assert_equal "Thu Feb 29 13:45:30 2024 | 1709181930 | 000 000000000 000 | 1709181930000 | +09:00 | +09:00 | +09:00:00 | pm PM 01  1 060 08 09 2024 09 4",
                   d.strftime("%c | %s | %L %N %3N | %Q | %Z | %:z | %::z | %P %p %I %l %j %U %W %G %V %u")
      assert_equal "Thu Feb 29 13:45:30 +09:00 2024 29-FEB-2024 02/29/24 13:45:30 01:45:30 PM 13:45 13:45:30 02/29/24 2024-02-29 %Q",
                   d.strftime("%+ %v %x %X %r %R %T %D %F %%Q")
      assert_equal "2024-02-29T13:45:30+09:00", d.strftime
      assert_equal "+05:30:00|+05:30|+05:30", DateTime.new(2024, 3, 5, 1, 2, 3, "+05:30").strftime("%::z|%:::z|%Z")
      assert_equal "+09", d.strftime("%:::z")
    end

    def test_date_typed_datetime
      x = DateTime.new(2024, 2, 29, 23, 30) #: Date
      assert_equal "2024-02-29T23:30:00+00:00", x.to_s
      assert_equal "2024-02-29T23:30:00+00:00", x.strftime
      assert_equal "2024-02-29T23:30:00+00:00", x.iso8601
      assert_equal "2024-03-01T23:30:00+00:00", (x + 1).to_s
      assert_equal "DateTime", (x >> 1).class.to_s
    end

    def test_parsers
      assert_equal "#<DateTime: 2024-03-01T10:00:00+00:00 ((2460371j,36000s,0n),+0s,2299161j)>", DateTime.parse("2024-03-01 10:00").inspect
      assert_equal "2024-03-05T15:04:00+00:00", DateTime.parse("March 5, 2024 3:04pm").to_s
      assert_equal "2001-02-03T04:05:06+07:00", DateTime.parse("Sat, 03 Feb 2001 04:05:06 +0700").to_s
      assert_equal "#<DateTime: 2024-03-01T10:00:00-05:00 ((2460371j,54000s,123000000n),-18000s,2299161j)>", DateTime.parse("2024-03-01T10:00:00.123-05:00").inspect
      assert_equal "2024-03-01 00:00:00 UTC", DateTime.parse("2024-03-01T00:00:00Z").to_time.utc.to_s
      assert_equal "2024-03-01T00:00:00+00:00", DateTime.iso8601("2024-03-01").to_s
      assert_equal "2024-03-01T10:11:12+00:00", DateTime.iso8601("20240301T101112Z").to_s
      assert_equal "2024-03-01T09:00:00+00:00", DateTime.iso8601("2024-03-01T10:00:00+01:00").new_offset(0).to_s
      assert_equal "2024-03-01T10:00:00+01:00", DateTime.rfc3339("2024-03-01T10:00:00+01:00").to_s
      assert_equal "2024-02-29T04:45:30+00:00", DateTime.httpdate("Thu, 29 Feb 2024 04:45:30 GMT").to_s
      assert_equal "2024-02-29T13:45:30+09:00", DateTime.rfc2822("Thu, 29 Feb 2024 13:45:30 +0900").to_s
      assert_equal "2024-02-29T13:45:30+09:00", DateTime.jisx0301("R06.02.29T13:45:30+09:00").to_s
      assert_equal "2024-02-29T13:45:30+09:00", DateTime.xmlschema("2024-02-29T13:45:30+09:00").to_s
      %w[nope 2024-03-01].each do |s|
        assert_raises(Date::Error) { DateTime.rfc3339(s) }
      end
      assert_raises(Date::Error) { DateTime.parse("nope") }
      assert_raises(Date::Error) { DateTime.iso8601("garbage") }
    end

    def test_strptime
      assert_equal "2024-03-01T10:11:12+00:00", DateTime.strptime("2024-03-01T10:11:12+00:00").to_s
      assert_equal "2024-03-01T10:11:12+09:00", DateTime.strptime("2024-03-01 10:11:12 +0900", "%Y-%m-%d %H:%M:%S %z").to_s
      assert_equal "2024-03-01T13:02:03+00:00", DateTime.strptime("03/01/24 01:02:03 PM", "%D %I:%M:%S %p").to_s
      assert_equal "2024-02-29T04:45:30+00:00", DateTime.strptime("1709181930", "%s").to_s
      assert_raises(Date::Error) { DateTime.strptime("2024-03-01T10:11:12") }
    end

    def test_conversions
      d = dt
      assert_equal true, d.to_datetime.equal?(d)
      assert_equal "#<Date: 2024-02-29 ((2460370j,0s,0n),+0s,2299161j)>", d.to_date.inspect
      assert_equal "Date", d.to_date.class.to_s
      assert_equal "2024-02-29 13:45:30 +0900", d.to_time.to_s
      assert_equal 32_400, d.to_time.utc_offset
      assert_equal "2024-01-01 00:00:00 +0000", DateTime.new(2024, 1, 1).to_time.to_s
      assert_equal "#<DateTime: 2024-01-01T00:00:00+00:00 ((2460311j,0s,0n),+0s,2299161j)>", Date.new(2024, 1, 1).to_datetime.inspect
      assert_equal "#<DateTime: 1970-01-01T00:00:01+00:00 ((2440588j,1s,500000000n),+0s,2299161j)>", Time.at(1.5).utc.to_datetime.inspect
      assert_equal "2024-01-01T10:00:00+09:00", Time.new(2024, 1, 1, 10, 0, 0, "+09:00").to_datetime.to_s
    end

    def test_now
      n = DateTime.now
      assert_equal "DateTime", n.class.to_s
      assert_equal true, n.offset == Rational(Time.now.utc_offset, 86_400)
      assert_equal true, (n - DateTime.now).abs < Rational(1, 86_400)
      assert_equal true, n.year >= 2024
    end

    # A Date carries a day fraction after fractional arithmetic, as MRI's.
    def test_date_fractions
      half = Date.new(2024, 1, 1) + 0.5
      assert_equal "#<Date: 2024-01-01 ((2460311j,43200s,0n),+0s,2299161j)>", half.inspect
      assert_equal "#<Date: 2024-01-01 ((2460311j,43200s,0n),+0s,2299161j)>", (Date.new(2024, 1, 1) + Rational(1, 2)).inspect
      assert_equal false, half == Date.new(2024, 1, 1)
      assert_equal "#<Date: 2024-01-02 ((2460312j,0s,0n),+0s,2299161j)>", (half + 0.5).inspect
      assert_equal Rational(1, 4), (Date.new(2024, 1, 1) + Rational(1, 4)).day_fraction
      assert_equal " 5-MAR-2024|+00:00|+0000|+00:00|1709596800000|1709596800", Date.new(2024, 3, 5).strftime("%v|%Z|%z|%:z|%Q|%s")
      assert_equal "Tue, 5 Mar 2024 00:00:00 +0000", Date.new(2024, 3, 5).rfc2822
    end
  end
end
