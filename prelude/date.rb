# rbs_inline: enabled

# A civil day number (jd) on the proleptic Gregorian calendar, plus the seconds (df) and nanoseconds (sf) into that day and a UTC offset in seconds (of), as MRI's; MRI's Julian dates before 1582 are not modelled. A struct class, not a @go_type, so DateTime can subclass it (decision 133).
class Date < Object
  include Comparable

  # @rbs @jd: Integer
  # @rbs @df: Integer
  # @rbs @sf: Integer
  # @rbs @of: Integer

  class Error < ArgumentError; end

  DAYNAMES = %w[Sunday Monday Tuesday Wednesday Thursday Friday Saturday] #: Array[String]
  ABBR_DAYNAMES = %w[Sun Mon Tue Wed Thu Fri Sat] #: Array[String]
  MONTHNAMES = [nil, "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"] #: Array[String?]
  ABBR_MONTHNAMES = [nil, "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] #: Array[String?]

  #: (?Integer, ?Integer, ?Integer) -> Date
  def self.new(y = -4712, m = 1, d = 1) = %x{ return rbNewDate(rbDateCivil(int(y), int(m), int(d))) }

  #: (?Integer, ?Integer, ?Integer) -> Date
  def self.civil(y = -4712, m = 1, d = 1) = new(y, m, d)

  #: (Integer) -> Date
  def self.jd(n) = %x{ return rbNewDate(rbDateParts{jd: int(n)}) }

  #: () -> Date
  def self.today = %x{
    t := time.Now()
    return rbNewDate(rbDateCivil(t.Year(), int(t.Month()), t.Day()))
  }

  #: (Integer, Integer, Integer) -> bool
  def self.valid_date?(y, m, d) = %x{
    _, ok := rbCivilJD(int(y), int(m), int(d))
    return Boolean(ok)
  }

  #: (Integer, Integer, Integer) -> bool
  def self.valid_civil?(y, m, d) = valid_date?(y, m, d)

  #: (Integer) -> bool
  def self.leap?(y) = (y % 4 == 0 && y % 100 != 0) || y % 400 == 0

  #: (Integer) -> bool
  def self.gregorian_leap?(y) = leap?(y)

  #: (String) -> Date
  def self.parse(s) = %x{
    y, m, d, ok := rbDateParse(string(s))
    if !ok {
      panic(rbDateInvalid())
    }
    return rbNewDate(rbDateCivil(y, m, d))
  }

  #: (String) -> Date
  def self.iso8601(s) = parse(s)

  #: (String) -> Date
  def self.rfc3339(s) = parse(s)

  #: (String) -> Date
  def self.httpdate(s) = parse(s)

  #: (String) -> Date
  def self.jisx0301(s) = %x{
    y, m, d, ok := rbJISX0301Parse(string(s))
    if !ok {
      panic(rbDateInvalid())
    }
    return rbNewDate(rbDateCivil(y, m, d))
  }

  #: (String, ?String) -> Date
  def self.strptime(s, fmt = "%F") = %x{
    y, m, d, ok := rbDateStrptime(string(s), string(fmt))
    if !ok {
      panic(rbDateInvalid())
    }
    return rbNewDate(rbDateCivil(y, m, d))
  }

  #: () -> Integer
  def jd = @jd

  #: () -> Integer
  def mjd = jd - 2400001

  #: () -> Integer
  def year = %x{ Integer(rbJDTime(int(self._Date().jd)).Year()) }

  #: () -> Integer
  def month = %x{ Integer(rbJDTime(int(self._Date().jd)).Month()) }

  #: () -> Integer
  def mon = month

  #: () -> Integer
  def day = %x{ Integer(rbJDTime(int(self._Date().jd)).Day()) }

  #: () -> Integer
  def mday = day

  #: () -> Integer
  def wday = (jd + 1) % 7

  #: () -> Integer
  def yday = %x{ Integer(rbJDTime(int(self._Date().jd)).YearDay()) }

  #: () -> Integer
  def cwday = wday == 0 ? 7 : wday

  #: () -> Integer
  def cweek = %x{
    _, w := rbJDTime(int(self._Date().jd)).ISOWeek()
    return Integer(w)
  }

  #: () -> Integer
  def cwyear = %x{
    y, _ := rbJDTime(int(self._Date().jd)).ISOWeek()
    return Integer(y)
  }

  #: () -> Rational
  def day_fraction = Rational(@df * 1_000_000_000 + @sf, 86_400_000_000_000)

  #: () -> bool
  def leap? = Date.leap?(year)

  #: () -> bool
  def sunday? = wday == 0

  #: () -> bool
  def monday? = wday == 1

  #: () -> bool
  def tuesday? = wday == 2

  #: () -> bool
  def wednesday? = wday == 3

  #: () -> bool
  def thursday? = wday == 4

  #: () -> bool
  def friday? = wday == 5

  #: () -> bool
  def saturday? = wday == 6

  #: (Integer) -> Date
  def +(n) = %x{ return rbNewDate(rbDateAddDays(rbDateGet(self._Date()), n)) }

  #: (Float) -> Date
  def __plus_float(n) = %x{ return rbNewDate(rbDateAddDays(rbDateGet(self._Date()), n)) }

  #: (Rational) -> Date
  def __plus_rational(n) = %x{ return rbNewDate(rbDateAddDays(rbDateGet(self._Date()), n)) }

  #: (Integer) -> Date
  def -(n) = self + -n

  #: (Float) -> Date
  def __minus_float(n) = __plus_float(-n)

  #: (Rational) -> Date
  def __minus_rational(n) = __plus_rational(-n)

  #: (Date) -> Rational
  def __minus_date(o) = %x{ return rbDateMinus(self._Date(), o._Date()) }

  #: (DateTime) -> Rational
  def __minus_date_time(o) = %x{ return rbDateMinus(self._Date(), o._Date()) }

  # Month arithmetic clamps the day to the target month's end, as MRI.
  #: (Integer) -> Date
  def >>(n) = %x{ return rbNewDate(rbDateAddMonths(rbDateGet(self._Date()), int(n))) }

  #: (Integer) -> Date
  def <<(n) = self >> -n

  #: (?Integer) -> Date
  def next_day(n = 1) = self + n

  #: (?Integer) -> Date
  def prev_day(n = 1) = self - n

  #: (?Integer) -> Date
  def next_month(n = 1) = self >> n

  #: (?Integer) -> Date
  def prev_month(n = 1) = self << n

  #: (?Integer) -> Date
  def next_year(n = 1) = self >> (n * 12)

  #: (?Integer) -> Date
  def prev_year(n = 1) = self << (n * 12)

  #: () -> Date
  def succ = self + 1

  #: () -> Date
  def next = self + 1

  #: (Date) { (Date) -> void } -> void
  def upto(max, &) = step(max, 1, &)

  #: (Date) { (Date) -> void } -> void
  def downto(min, &) = step(min, -1, &)

  #: (Date, ?Integer) { (Date) -> void } -> void
  def step(limit, by = 1)
    raise ArgumentError, "step can't be 0" if by == 0

    d = self #: Date
    while by > 0 ? d <= limit : d >= limit
      yield d
      d += by
    end
  end

  #: (Date) -> Integer
  def <=>(o) = %x{ Integer(rbDateCmp(self._Date(), o._Date())) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := rbUnbox(other).(DateI)
    return Boolean(ok && rbDateCmp(self._Date(), o._Date()) == 0)
  }

  # Same calendar day, each in its own offset.
  #: (untyped) -> bool
  def ===(other) = %x{
    o, ok := rbUnbox(other).(DateI)
    return Boolean(ok && o._Date().jd == self._Date().jd)
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{ rbDateHash(self._Date()) }

  #: () -> bool
  def frozen? = true

  # nil, not a literal default, so a Date-typed DateTime gets DateTime's: literal defaults are filled in at the call site (decision 8).
  #: () -> String
  def __strftime_default = "%F"

  #: (?String?) -> String
  def strftime(fmt = nil) = __strftime(fmt || __strftime_default)

  #: (String) -> String
  def __strftime(fmt) = %x{ String(rbDateStrftime(rbDateGet(self._Date()), string(fmt))) }

  #: () -> String
  def to_s = strftime("%Y-%m-%d")

  #: () -> String
  def iso8601 = strftime("%Y-%m-%d")

  #: () -> String
  def xmlschema = iso8601

  #: () -> String
  def rfc3339 = strftime("%Y-%m-%dT%H:%M:%S%:z")

  #: () -> String
  def httpdate = %x{ String(rbStrftime(rbDateGoTime(rbDateGet(self._Date())).UTC(), "%a, %d %b %Y %H:%M:%S GMT", true)) }

  #: () -> String
  def rfc2822 = strftime("%a, %-d %b %Y %H:%M:%S %z")

  #: () -> String
  def rfc822 = rfc2822

  #: () -> String
  def jisx0301 = %x{ String(rbJISX0301(int(self._Date().jd))) }

  #: () -> String
  def inspect = %x{ String(rbDateInspect("Date", rbDateGet(self._Date()), rbStrftime(rbJDTime(int(self._Date().jd)), "%Y-%m-%d", true))) }

  #: () -> Time
  def to_time = Time.local(year, month, day)

  #: () -> Date
  def to_date = self

  #: () -> DateTime
  def to_datetime = %x{ return rbNewDateTime(rbDateGet(self._Date())) }
end

# A Date with a time of day and an offset (decision 133). Arithmetic is in days, as Date's.
class DateTime < Date
  #: (?Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?untyped, ?untyped) -> DateTime
  def self.new(y = -4712, m = 1, d = 1, h = 0, mi = 0, s = 0, of = 0) = %x{
    sec, ns := rbDateSecArg(s)
    return rbNewDateTime(rbDateTimeOf(int(y), int(m), int(d), int(h), int(mi), sec, ns, rbDateOffsetArg(of)))
  }

  #: (?Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?untyped, ?untyped) -> DateTime
  def self.civil(y = -4712, m = 1, d = 1, h = 0, mi = 0, s = 0, of = 0) = new(y, m, d, h, mi, s, of)

  #: (?Integer, ?Integer, ?Integer, ?untyped, ?untyped) -> DateTime
  def self.jd(n = 0, h = 0, mi = 0, s = 0, of = 0) = %x{
    sec, ns := rbDateSecArg(s)
    t := rbJDTime(int(n))
    return rbNewDateTime(rbDateTimeOf(t.Year(), int(t.Month()), t.Day(), int(h), int(mi), sec, ns, rbDateOffsetArg(of)))
  }

  #: () -> DateTime
  def self.now = %x{ return rbNewDateTime(rbDateNow()) }

  # MRI undefines it; inherited, it would answer a Date.
  #: () -> DateTime
  def self.today = raise(NoMethodError, "undefined method 'today' for class DateTime")

  #: (String) -> DateTime
  def self.parse(s) = %x{ return rbNewDateTime(rbDateTimeParse(string(s))) }

  #: (String) -> DateTime
  def self.iso8601(s) = %x{ return rbNewDateTime(rbDateTimeISO8601(string(s))) }

  #: (String) -> DateTime
  def self.xmlschema(s) = iso8601(s)

  #: (String) -> DateTime
  def self.rfc3339(s) = %x{ return rbNewDateTime(rbDateTimeFormat(string(s), rbDateRFC3339)) }

  #: (String) -> DateTime
  def self.httpdate(s) = %x{ return rbNewDateTime(rbDateTimeFormat(string(s), rbDateHTTP)) }

  #: (String) -> DateTime
  def self.rfc2822(s) = %x{ return rbNewDateTime(rbDateTimeFormat(string(s), rbDateRFC2822)) }

  #: (String) -> DateTime
  def self.rfc822(s) = rfc2822(s)

  #: (String) -> DateTime
  def self.jisx0301(s) = %x{ return rbNewDateTime(rbDateTimeJISX0301(string(s))) }

  #: (String, ?String) -> DateTime
  def self.strptime(s, fmt = "%FT%T%z") = %x{ return rbNewDateTime(rbDateTimeStrptime(string(s), string(fmt))) }

  #: () -> Integer
  def hour = @df / 3600

  #: () -> Integer
  def minute = @df % 3600 / 60

  #: () -> Integer
  def min = minute

  #: () -> Integer
  def second = @df % 60

  #: () -> Integer
  def sec = second

  #: () -> Rational
  def sec_fraction = Rational(@sf, 1_000_000_000)

  #: () -> Rational
  def second_fraction = sec_fraction

  #: () -> Rational
  def offset = Rational(@of, 86_400)

  #: () -> String
  def zone = %x{ String(rbDateZoneName(int(self._Date().of))) }

  #: (?untyped) -> DateTime
  def new_offset(of = 0) = %x{
    p := rbDateGet(self._Date())
    sec, ns := p.abs()
    return rbNewDateTime(rbDateAt(sec, ns, rbDateOffsetArg(of)))
  }

  #: (Integer) -> DateTime
  def +(n) = %x{ return rbNewDateTime(rbDateAddDays(rbDateGet(self._Date()), n)) }

  #: (Float) -> DateTime
  def __plus_float(n) = %x{ return rbNewDateTime(rbDateAddDays(rbDateGet(self._Date()), n)) }

  #: (Rational) -> DateTime
  def __plus_rational(n) = %x{ return rbNewDateTime(rbDateAddDays(rbDateGet(self._Date()), n)) }

  #: (Integer) -> DateTime
  def -(n) = self + -n

  #: (Float) -> DateTime
  def __minus_float(n) = __plus_float(-n)

  #: (Rational) -> DateTime
  def __minus_rational(n) = __plus_rational(-n)

  #: (Integer) -> DateTime
  def >>(n) = %x{ return rbNewDateTime(rbDateAddMonths(rbDateGet(self._Date()), int(n))) }

  #: (Integer) -> DateTime
  def <<(n) = self >> -n

  #: (?Integer) -> DateTime
  def next_day(n = 1) = self + n

  #: (?Integer) -> DateTime
  def prev_day(n = 1) = self - n

  #: (?Integer) -> DateTime
  def next_month(n = 1) = self >> n

  #: (?Integer) -> DateTime
  def prev_month(n = 1) = self << n

  #: (?Integer) -> DateTime
  def next_year(n = 1) = self >> (n * 12)

  #: (?Integer) -> DateTime
  def prev_year(n = 1) = self << (n * 12)

  #: () -> DateTime
  def succ = self + 1

  #: () -> DateTime
  def next = self + 1

  #: () -> String
  def __strftime_default = "%FT%T%:z"

  #: () -> String
  def to_s = strftime("%Y-%m-%dT%H:%M:%S%:z")

  #: (?Integer) -> String
  def iso8601(n = 0) = %x{
    p := rbDateGet(self._Date())
    return String(rbDateStrftime(p, "%Y-%m-%dT%H:%M:%S") + rbDateFrac(p, int(n)) + rbDateZoneName(p.of))
  }

  #: (?Integer) -> String
  def xmlschema(n = 0) = iso8601(n)

  #: (?Integer) -> String
  def rfc3339(n = 0) = iso8601(n)

  #: (?Integer) -> String
  def jisx0301(n = 0) = %x{
    p := rbDateGet(self._Date())
    return String(rbJISX0301(p.jd) + rbDateStrftime(p, "T%H:%M:%S") + rbDateFrac(p, int(n)) + rbDateZoneName(p.of))
  }

  #: () -> String
  def inspect = %x{
    p := rbDateGet(self._Date())
    return String(rbDateInspect("DateTime", p, rbDateStrftime(p, "%Y-%m-%dT%H:%M:%S%:z")))
  }

  #: () -> Time
  def to_time = %x{ return &Time{t: rbDateGoTime(rbDateGet(self._Date()))} }

  #: () -> Date
  def to_date = Date.jd(jd)

  #: () -> DateTime
  def to_datetime = self
end

class Time
  #: () -> Date
  def to_date = Date.new(year, month, day)

  #: () -> DateTime
  def to_datetime = %x{ return rbNewDateTime(rbDateFromGoTime(self.t)) }
end
