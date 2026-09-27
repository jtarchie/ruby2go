# rbs_inline: enabled

# A day number (jd) on the proleptic Gregorian calendar; MRI's Julian dates before 1582 are not modelled.
# @go_type struct { jd int }
class Date < Object
  include Comparable

  class Error < ArgumentError; end

  DAYNAMES = %w[Sunday Monday Tuesday Wednesday Thursday Friday Saturday] #: Array[String]
  ABBR_DAYNAMES = %w[Sun Mon Tue Wed Thu Fri Sat] #: Array[String]
  MONTHNAMES = [nil, "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"] #: Array[String?]
  ABBR_MONTHNAMES = [nil, "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] #: Array[String?]

  #: (?Integer, ?Integer, ?Integer) -> Date
  def self.new(y = -4712, m = 1, d = 1) = %x{
    jd, ok := rbCivilJD(int(y), int(m), int(d))
    if !ok {
      panic(NewDate_Error(Ref(String("invalid date"))))
    }
    return &Date{jd: jd}
  }

  #: (?Integer, ?Integer, ?Integer) -> Date
  def self.civil(y = -4712, m = 1, d = 1) = new(y, m, d)

  #: (Integer) -> Date
  def self.jd(n) = %x{ return &Date{jd: int(n)} }

  #: () -> Date
  def self.today = %x{
    t := time.Now()
    jd, _ := rbCivilJD(t.Year(), int(t.Month()), t.Day())
    return &Date{jd: jd}
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
      panic(NewDate_Error(Ref(String("invalid date"))))
    }
    return Date_class.New(Integer(y), Integer(m), Integer(d))
  }

  #: (String) -> Date
  def self.iso8601(s) = parse(s)

  #: (String, ?String) -> Date
  def self.strptime(s, fmt = "%F") = %x{
    y, m, d, ok := rbDateStrptime(string(s), string(fmt))
    if !ok {
      panic(NewDate_Error(Ref(String("invalid date"))))
    }
    return Date_class.New(Integer(y), Integer(m), Integer(d))
  }

  #: () -> Integer
  def jd = %x{ Integer(self.jd) }

  #: () -> Integer
  def mjd = jd - 2400001

  #: () -> Integer
  def year = %x{ Integer(rbJDTime(self.jd).Year()) }

  #: () -> Integer
  def month = %x{ Integer(rbJDTime(self.jd).Month()) }

  #: () -> Integer
  def mon = month

  #: () -> Integer
  def day = %x{ Integer(rbJDTime(self.jd).Day()) }

  #: () -> Integer
  def mday = day

  #: () -> Integer
  def wday = (jd + 1) % 7

  #: () -> Integer
  def yday = %x{ Integer(rbJDTime(self.jd).YearDay()) }

  #: () -> Integer
  def cwday = wday == 0 ? 7 : wday

  #: () -> Integer
  def cweek = %x{
    _, w := rbJDTime(self.jd).ISOWeek()
    return Integer(w)
  }

  #: () -> Integer
  def cwyear = %x{
    y, _ := rbJDTime(self.jd).ISOWeek()
    return Integer(y)
  }

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
  def +(n) = Date.jd(jd + n)

  #: (Integer) -> Date
  def -(n) = Date.jd(jd - n)

  #: (Date) -> Rational
  def __minus_date(o) = Rational(jd - o.jd, 1)

  # Month arithmetic clamps the day to the target month's end, as MRI.
  #: (Integer) -> Date
  def >>(n) = %x{
    t := rbJDTime(self.jd)
    ms := t.Year()*12 + int(t.Month()) - 1 + int(n)
    y, m := ms/12, ms%12+1
    if ms < 0 && ms%12 != 0 {
      y, m = (ms-11)/12, ms-((ms-11)/12)*12+1
    }
    d := min(t.Day(), rbMonthDays(y, m))
    jd, _ := rbCivilJD(y, m, d)
    return &Date{jd: jd}
  }

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
  def upto(max) = %x{
    return func(yield func(*Date) bool) {
      for j := self.jd; j <= max.jd; j++ {
        if !yield(&Date{jd: j}) {
          return
        }
      }
    }
  }

  #: (Date) { (Date) -> void } -> void
  def downto(min) = %x{
    return func(yield func(*Date) bool) {
      for j := self.jd; j >= min.jd; j-- {
        if !yield(&Date{jd: j}) {
          return
        }
      }
    }
  }

  #: (Date, ?Integer) { (Date) -> void } -> void
  def step(limit, by = 1) = %x{
    return func(yield func(*Date) bool) {
      if by == 0 {
        panic(NewArgumentError(Ref(String("step can't be 0"))))
      }
      for j := self.jd; (by > 0 && j <= limit.jd) || (by < 0 && j >= limit.jd); j += int(by) {
        if !yield(&Date{jd: j}) {
          return
        }
      }
    }
  }

  #: (Date) -> Integer
  def <=>(o) = %x{ Integer(cmp.Compare(self.jd, o.jd)) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Date)
    return Boolean(ok && o.jd == self.jd)
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = jd

  #: () -> bool
  def frozen? = true

  #: (?String) -> String
  def strftime(fmt = "%F") = %x{ String(rbStrftime(rbJDTime(self.jd), string(fmt), true)) }

  #: () -> String
  def to_s = strftime("%Y-%m-%d")

  #: () -> String
  def iso8601 = to_s

  #: () -> String
  def inspect = "#<Date: #{self} ((#{jd}j,0s,0n),+0s,2299161j)>"

  #: () -> Time
  def to_time = Time.local(year, month, day)

  #: () -> Date
  def to_date = self
end

class Time
  #: () -> Date
  def to_date = Date.new(year, month, day)
end
