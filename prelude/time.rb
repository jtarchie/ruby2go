# rbs_inline: enabled

# utc marks MRI's UTC mode, which prints "UTC" rather than "+0000".
# @go_type struct { t time.Time; utc bool }
class Time < Object
  include Comparable

  #: (?untyped) -> Time
  def self.now(zone = nil) = %x{
    loc, utc := rbTimeZoneArg(zone, time.Local)
    return &Time{t: time.Now().In(loc), utc: utc}
  }

  #: (Integer, ?untyped) -> Time
  def self.at(sec, zone = nil) = %x{
    loc, utc := rbTimeZoneArg(zone, time.Local)
    return &Time{t: time.Unix(int64(sec), 0).In(loc), utc: utc}
  }

  #: (Float, ?untyped) -> Time
  def self.__at_float(sec, zone = nil) = %x{
    loc, utc := rbTimeZoneArg(zone, time.Local)
    return &Time{t: rbTimeFromFloat(float64(sec)).In(loc), utc: utc}
  }

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

  #: (Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?Integer, ?untyped) -> Time
  def self.new(y, mo = 1, d = 1, h = 0, mi = 0, s = 0, zone = nil) = %x{
    rbTimeArgs(y, mo, d, h, mi, s)
    loc, utc := rbTimeZoneArg(zone, time.Local)
    return &Time{t: time.Date(int(y), time.Month(mo), int(d), int(h), int(mi), int(s), 0, loc), utc: utc}
  }

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

  # The fraction of a second: a Rational, or Integer 0 on a whole second, as MRI's.
  #: () -> untyped
  def subsec
    return 0 if nsec.zero?

    Rational(nsec, 1_000_000_000)
  end

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

  #: (?untyped) -> Time
  def localtime(off = nil) = %x{
    loc, utc := rbTimeZoneArg(off, time.Local)
    self.t, self.utc = self.t.In(loc), utc
    return self
  }

  #: () -> Time
  def getutc = %x{ return &Time{t: self.t.UTC(), utc: true} }

  #: () -> Time
  def getgm = getutc

  #: () -> bool
  def gmt? = utc?

  #: () -> Integer
  def gmtoff = utc_offset

  #: (?untyped) -> Time
  def getlocal(off = nil) = %x{
    loc, utc := rbTimeZoneArg(off, time.Local)
    return &Time{t: self.t.In(loc), utc: utc}
  }

  #: () -> Integer
  def to_i = %x{ Integer(self.t.Unix()) }

  #: () -> Integer
  def tv_sec = to_i

  #: () -> Integer
  def tv_usec = usec

  #: () -> Integer
  def tv_nsec = nsec

  #: () -> Float
  def to_f = %x{ Float(float64(self.t.UnixNano()) / 1e9) }

  #: () -> bool
  def dst? = %x{ Boolean(self.t.IsDST()) }

  #: () -> bool
  def isdst = dst?

  #: (?Integer) -> Time
  def round(digits = 0) = %x{ return &Time{t: rbTimeRound(self.t, int(digits), 0), utc: self.utc} }

  #: (?Integer) -> Time
  def floor(digits = 0) = %x{ return &Time{t: rbTimeRound(self.t, int(digits), -1), utc: self.utc} }

  #: (?Integer) -> Time
  def ceil(digits = 0) = %x{ return &Time{t: rbTimeRound(self.t, int(digits), 1), utc: self.utc} }

  #: () -> String
  def ctime = %x{ String(self.t.Format(time.ANSIC)) }

  #: () -> String
  def asctime = ctime

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
  def strftime(fmt) = __strftime(fmt, nil, nil, true)

  DAYS__ = %w[Sunday Monday Tuesday Wednesday Thursday Friday Saturday] #: Array[String]
  MONTHS__ = %w[January February March April May June July August September October November December] #: Array[String]

  # MRI's strftime (decision 158): each token is a directive (%, flags - 0 _ ^ # :, a width, an E/O modifier
  # MRI ignores, the conversion) or literal text. plus is what %+ expands to (Date's; Time prints it as is), q
  # the milliseconds %Q prints (Date's), strict whether a directive missing its conversion is an error (Time)
  # or printed as is (Date).
  #: (String, String?, Integer?, bool) -> String
  def __strftime(fmt, plus, q, strict)
    fmt.scan(/%[-0_^#:]*\d*[EO]?.?|[^%]+/m).map do |tok|
      next tok unless tok.start_with?("%")
      __strftime_token(tok, plus, q, strict) || raise(ArgumentError, "invalid format: #{fmt}")
    end.join
  end

  # One directive; nil for one strict rejects.
  #: (String, String?, Integer?, bool) -> String?
  def __strftime_token(tok, plus, q, strict)
    m = tok.match(/\A%([-0_^#:]*)(\d*)[EO]?(.)?\z/m)
    return tok if m.nil?
    flags = m[1] || ""
    digits = m[2] || ""
    width = digits.empty? ? -1 : digits.to_i
    c = m[3]
    if c.nil?
      return nil if strict && tok.match?(/\A%[-0_^#]*\d*\z/)
      return tok
    end
    conv = c
    return tok if flags.include?(":") && conv != "z"
    __strftime_conv(conv, flags, width, tok, plus, q)
  end

  #: (String, String, Integer, String, String?, Integer?) -> String
  def __strftime_conv(conv, flags, width, tok, plus, q)
    case conv
    when "Y" then __sf_num(year, year < 0 ? 5 : 4, "0", flags, width)
    when "C" then __sf_num(year.div(100), 2, "0", flags, width)
    when "y" then __sf_num(year % 100, 2, "0", flags, width)
    when "m" then __sf_num(month, 2, "0", flags, width)
    when "d" then __sf_num(day, 2, "0", flags, width)
    when "e" then __sf_num(day, 2, " ", flags, width)
    when "j" then __sf_num(yday, 3, "0", flags, width)
    when "H" then __sf_num(hour, 2, "0", flags, width)
    when "k" then __sf_num(hour, 2, " ", flags, width)
    when "I" then __sf_num(__hour12, 2, "0", flags, width)
    when "l" then __sf_num(__hour12, 2, " ", flags, width)
    when "M" then __sf_num(min, 2, "0", flags, width)
    when "S" then __sf_num(sec, 2, "0", flags, width)
    when "L", "N"
      n = conv == "N" ? 9 : 3
      n = width if width > 0
      s = nsec.to_s.rjust(9, "0")
      s += "0" while s.size < n
      s[0, n] || ""
    when "z" then __sf_offset(flags.count(":"))
    when "Z" then __sf_text(zone, flags, width)
    when "A" then __sf_text(DAYS__.fetch(wday), flags, width)
    when "a" then __sf_text(DAYS__.fetch(wday)[0, 3] || "", flags, width)
    when "B" then __sf_text(MONTHS__.fetch(month - 1), flags, width)
    when "b", "h" then __sf_text(MONTHS__.fetch(month - 1)[0, 3] || "", flags, width)
    when "p"
      ampm = hour >= 12 ? "PM" : "AM"
      flags.include?("#") ? __sf_text(ampm.downcase, "", width) : __sf_text(ampm, flags, width)
    when "P" then __sf_text(hour >= 12 ? "pm" : "am", flags, width)
    when "u" then __sf_num(wday.zero? ? 7 : wday, 1, "0", flags, width)
    when "w" then __sf_num(wday, 1, "0", flags, width)
    when "U" then __sf_num((yday + 6 - wday) / 7, 2, "0", flags, width)
    when "W" then __sf_num((yday + 6 - (wday + 6) % 7) / 7, 2, "0", flags, width)
    when "G"
      y = __iso_week.fetch(0)
      __sf_num(y, y < 0 ? 5 : 4, "0", flags, width)
    when "g" then __sf_num(__iso_week.fetch(0) % 100, 2, "0", flags, width)
    when "V" then __sf_num(__iso_week.fetch(1), 2, "0", flags, width)
    when "s" then __sf_num(to_i, 1, "0", flags, width)
    when "n" then "\n"
    when "t" then "\t"
    when "%" then __sf_text("%", flags, width)
    when "F" then __sf_text(__strftime("%Y-%m-%d", plus, q, false), flags, width)
    when "T", "X" then __sf_text(__strftime("%H:%M:%S", plus, q, false), flags, width)
    when "D", "x" then __sf_text(__strftime("%m/%d/%y", plus, q, false), flags, width)
    when "R" then __sf_text(__strftime("%H:%M", plus, q, false), flags, width)
    when "r" then __sf_text(__strftime("%I:%M:%S %p", plus, q, false), flags, width)
    when "c" then __sf_text(__strftime("%a %b %e %H:%M:%S %Y", plus, q, false), flags, width)
    when "v" then __sf_text(__strftime("%e-%^b-%4Y", plus, q, false), flags, width)
    when "+"
      return tok unless plus
      __sf_text(__strftime(plus, nil, q, false), flags, width)
    when "Q"
      return tok unless q
      __sf_num(q, 1, "0", flags, width)
    else tok
    end
  end

  #: () -> Integer
  def __hour12
    h = hour % 12
    h.zero? ? 12 : h
  end

  # A number padded to w (the directive's width when given) with pad; - drops the padding, _ pads with
  # spaces, 0 with zeros. A sign counts against the width, as MRI's.
  #: (Integer, Integer, String, String, Integer) -> String
  def __sf_num(v, w, pad, flags, width)
    w = width if width >= 0
    if flags.include?("-")
      w = 0
    elsif flags.include?("_")
      pad = " "
    elsif flags.include?("0")
      pad = "0"
    end
    w -= 1 if v < 0
    s = v.abs.to_s.rjust(w, pad)
    v < 0 ? "-#{s}" : s
  end

  # Text with ^ (upcase), # (swap case) and the width (space padded, 0 for zeros, - for none).
  #: (String, String, Integer) -> String
  def __sf_text(s, flags, width)
    s = s.upcase if flags.include?("^")
    s = (s == s.upcase ? s.downcase : s.upcase) if flags.include?("#")
    return s if width <= 0 || flags.include?("-")
    s.rjust(width, flags.include?("0") ? "0" : " ")
  end

  # %z with 0, 1, 2 colons, or %:::z: hours, then minutes and seconds only when they are not zero.
  #: (Integer) -> String
  def __sf_offset(colons)
    off = utc_offset
    sign = off < 0 ? "-" : "+"
    off = off.abs
    h, m, s = off / 3600, off % 3600 / 60, off % 60
    case colons
    when 0 then format("%s%02d%02d", sign, h, m)
    when 1 then format("%s%02d:%02d", sign, h, m)
    when 2 then format("%s%02d:%02d:%02d", sign, h, m, s)
    else
      r = format("%s%02d", sign, h)
      r += format(":%02d", m) if m != 0 || s != 0
      r += format(":%02d", s) if s != 0
      r
    end
  end

  # [ISO 8601 week-based year, week]: the week holding this day's Thursday.
  #: () -> Array[Integer]
  def __iso_week
    thursday = yday - (wday + 6) % 7 + 3
    y = year
    if thursday < 1
      y -= 1
      thursday += __days_in_year(y)
    elsif thursday > __days_in_year(y)
      thursday -= __days_in_year(y)
      y += 1
    end
    [y, (thursday - 1) / 7 + 1]
  end

  #: (Integer) -> Integer
  def __days_in_year(y) = (y % 4).zero? && (y % 100 != 0 || (y % 400).zero?) ? 366 : 365

  #: () -> String
  def to_s = %x{ String(rbTimeToS(self.t, self.utc)) }

  #: () -> String
  def inspect = %x{ String(rbTimeInspect(self.t, self.utc)) }

  #: (?Integer) -> String
  def iso8601(digits = 0)
    s = strftime("%Y-%m-%dT%H:%M:%S")
    s += "." + strftime("%#{digits}N") if digits > 0
    utc? ? s + "Z" : s + strftime("%:z")
  end

  #: (?Integer) -> String
  def xmlschema(digits = 0) = iso8601(digits)

  #: () -> String
  def httpdate = %x{ String(self.t.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")) }

  # UTC mode prints -0000, as MRI's.
  #: () -> String
  def rfc2822 = %x{
    if self.utc {
      return String(self.t.Format("Mon, 02 Jan 2006 15:04:05 -0000"))
    }
    return String(self.t.Format("Mon, 02 Jan 2006 15:04:05 -0700"))
  }

  #: () -> String
  def rfc822 = rfc2822
end
