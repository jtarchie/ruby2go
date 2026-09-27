# rbs_inline: enabled

# utc marks MRI's UTC mode, which prints "UTC" rather than "+0000".
# @go_type struct { t time.Time; utc bool }
class Time < Object
  include Comparable

  #: () -> Time
  def self.now = %x{ return &Time{t: time.Now()} }

  #: (Integer) -> Time
  def self.at(sec) = %x{ return &Time{t: time.Unix(int64(sec), 0)} }

  #: (Float) -> Time
  def self.__at_float(sec) = %x{ return &Time{t: rbTimeFromFloat(float64(sec))} }

  #: (Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?Integer) -> Time
  def self.utc(y, mo = 1, d = 1, h = 0, mi = 0, s = 0) = %x{
    rbTimeArgs(y, mo, d, h, mi, s)
    return &Time{t: time.Date(int(y), time.Month(mo), int(d), int(h), int(mi), int(s), 0, time.UTC), utc: true}
  }

  #: (Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?Integer) -> Time
  def self.gm(y, mo = 1, d = 1, h = 0, mi = 0, s = 0) = utc(y, mo, d, h, mi, s)

  #: (Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?Integer) -> Time
  def self.local(y, mo = 1, d = 1, h = 0, mi = 0, s = 0) = %x{
    rbTimeArgs(y, mo, d, h, mi, s)
    return &Time{t: time.Date(int(y), time.Month(mo), int(d), int(h), int(mi), int(s), 0, time.Local)}
  }

  #: (Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?Integer) -> Time
  def self.mktime(y, mo = 1, d = 1, h = 0, mi = 0, s = 0) = local(y, mo, d, h, mi, s)

  #: (Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?Integer) -> Time
  def self.new(y, mo = 1, d = 1, h = 0, mi = 0, s = 0) = local(y, mo, d, h, mi, s)

  #: () -> Time
  def self.__new_0 = now

  #: () -> Integer
  def year = %x{ Integer(self.t.Year()) }

  #: () -> Integer
  def month = %x{ Integer(self.t.Month()) }

  #: () -> Integer
  def mon = month

  #: () -> Integer
  def day = %x{ Integer(self.t.Day()) }

  #: () -> Integer
  def mday = day

  #: () -> Integer
  def hour = %x{ Integer(self.t.Hour()) }

  #: () -> Integer
  def min = %x{ Integer(self.t.Minute()) }

  #: () -> Integer
  def sec = %x{ Integer(self.t.Second()) }

  #: () -> Integer
  def usec = %x{ Integer(self.t.Nanosecond() / 1000) }

  #: () -> Integer
  def nsec = %x{ Integer(self.t.Nanosecond()) }

  #: () -> Integer
  def wday = %x{ Integer(self.t.Weekday()) }

  #: () -> Integer
  def yday = %x{ Integer(self.t.YearDay()) }

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

  #: () -> Integer
  def utc_offset = %x{
    _, off := self.t.Zone()
    return Integer(off)
  }

  #: () -> Integer
  def gmt_offset = utc_offset

  #: () -> String
  def zone = %x{
    if self.utc {
      return "UTC"
    }
    name, _ := self.t.Zone()
    return String(name)
  }

  #: () -> bool
  def utc? = %x{ Boolean(self.utc) }

  # MRI converts the receiver in place.
  #: () -> Time
  def utc = %x{
    self.t, self.utc = self.t.UTC(), true
    return self
  }

  #: () -> Time
  def gmtime = utc

  #: () -> Time
  def localtime = %x{
    self.t, self.utc = self.t.Local(), false
    return self
  }

  #: () -> Time
  def getutc = %x{ return &Time{t: self.t.UTC(), utc: true} }

  #: () -> Time
  def getlocal = %x{ return &Time{t: self.t.Local()} }

  #: () -> Integer
  def to_i = %x{ Integer(self.t.Unix()) }

  #: () -> Float
  def to_f = %x{ Float(float64(self.t.UnixNano()) / 1e9) }

  #: (Integer) -> Time
  def +(secs) = %x{ return &Time{t: self.t.Add(time.Duration(secs) * time.Second), utc: self.utc} }

  #: (Float) -> Time
  def __plus_float(secs) = %x{ return &Time{t: self.t.Add(time.Duration(math.Round(float64(secs) * 1e9))), utc: self.utc} }

  #: (Integer) -> Time
  def -(secs) = %x{ return &Time{t: self.t.Add(-time.Duration(secs) * time.Second), utc: self.utc} }

  #: (Float) -> Time
  def __minus_float(secs) = %x{ return &Time{t: self.t.Add(-time.Duration(math.Round(float64(secs) * 1e9))), utc: self.utc} }

  #: (Time) -> Float
  def __minus_time(other) = %x{ Float(self.t.Sub(other.t).Seconds()) }

  #: (Time) -> Integer
  def <=>(other) = %x{ Integer(self.t.Compare(other.t)) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Time)
    return Boolean(ok && self.t.Equal(o.t))
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{ Integer(self.t.UnixNano()) }

  #: (String) -> String
  def strftime(fmt) = %x{ String(rbStrftime(self.t, string(fmt), self.utc)) }

  #: () -> String
  def to_s = %x{ String(rbTimeToS(self.t, self.utc)) }

  #: () -> String
  def inspect = %x{ String(rbTimeInspect(self.t, self.utc)) }

  #: (?Integer) -> String
  def iso8601(digits = 0) = %x{
    s := rbStrftime(self.t, "%Y-%m-%dT%H:%M:%S", self.utc)
    if digits > 0 {
      s += "." + rbStrftime(self.t, "%"+strconv.Itoa(int(digits))+"N", self.utc)
    }
    if self.utc {
      return String(s + "Z")
    }
    return String(s + rbStrftime(self.t, "%:z", self.utc))
  }

  #: (?Integer) -> String
  def xmlschema(digits = 0) = iso8601(digits)
end
