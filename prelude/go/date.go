//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"cmp"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// rbJDEpoch is the Julian Day Number of 1970-01-01.
const rbJDEpoch = 2440588

func rbJDTime(jd int) time.Time {
	return time.Unix(int64(jd-rbJDEpoch)*86400, 0).UTC()
}

func rbMonthDays(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// rbCivilJD validates y-m-d (negative m and d count from the end, as MRI) and returns its day number.
func rbCivilJD(y, m, d int) (int, bool) {
	if m < 0 {
		m += 13
	}
	if m < 1 || m > 12 {
		return 0, false
	}
	if d < 0 {
		d += rbMonthDays(y, m) + 1
	}
	if d < 1 || d > rbMonthDays(y, m) {
		return 0, false
	}
	secs := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC).Unix()
	days := secs / 86400
	if secs%86400 != 0 && secs < 0 {
		days--
	}
	return int(days) + rbJDEpoch, true
}

// rbAtoi is a regexp group's digits as an int; "" (an absent optional group) is 0.
func rbAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// rbDateParts is a Date's state (decision 133): local civil day, seconds and ns into it, UTC offset in seconds.
type rbDateParts struct{ jd, df, sf, of int }

func rbDateInvalid() any { return NewDate_Error(Ref(String("invalid date"))) }

func rbDateGet(d *Date) rbDateParts {
	return rbDateParts{int(d.jd), int(d.df), int(d.sf), int(d.of)}
}

func rbNewDate(p rbDateParts) *Date {
	return &Date{jd: Integer(p.jd), df: Integer(p.df), sf: Integer(p.sf), of: Integer(p.of)}
}

func rbNewDateTime(p rbDateParts) *DateTime {
	x := &DateTime{}
	d := x._Date()
	d.jd, d.df, d.sf, d.of = Integer(p.jd), Integer(p.df), Integer(p.sf), Integer(p.of)
	return x
}

// abs is the instant as UTC seconds since jd 0's midnight, plus nanoseconds.
func (p rbDateParts) abs() (sec, ns int) {
	return p.jd*86400 + p.df - p.of, p.sf
}

// rbDateAt is the instant sec/ns seen at offset of.
func rbDateAt(sec, ns, of int) rbDateParts {
	sec += ns / 1e9
	ns %= 1e9
	if ns < 0 {
		sec, ns = sec-1, ns+1e9
	}
	l := sec + of
	jd := l / 86400
	if l%86400 < 0 {
		jd--
	}
	return rbDateParts{jd, l - jd*86400, ns, of}
}

func rbDateCivil(y, m, d int) rbDateParts {
	jd, ok := rbCivilJD(y, m, d)
	if !ok {
		panic(rbDateInvalid())
	}
	return rbDateParts{jd: jd}
}

// rbDateTimeOf wraps negative h/mi/s and turns 24:00:00 into the next day, as MRI.
func rbDateTimeOf(y, m, d, h, mi, s, ns, of int) rbDateParts {
	jd, ok := rbCivilJD(y, m, d)
	if h < 0 {
		h += 24
	}
	if mi < 0 {
		mi += 60
	}
	if s < 0 {
		s += 60
	}
	if !ok || h < 0 || h > 24 || mi < 0 || mi > 59 || s < 0 || s > 59 || h == 24 && (mi > 0 || s > 0 || ns > 0) {
		panic(rbDateInvalid())
	}
	if h == 24 {
		jd, h = jd+1, 0
	}
	return rbDateParts{jd, h*3600 + mi*60 + s, ns, of}
}

// rbDateRat is an Integer, Float or Rational as an exact fraction.
func rbDateRat(v any, what string) *big.Rat {
	r := new(big.Rat)
	switch v := rbUnbox(v).(type) {
	case Integer:
		r.SetInt64(int64(v))
	case Float:
		if r.SetFloat64(float64(v)) == nil {
			panic(NewFloatDomainError(Ref(String(strconv.FormatFloat(float64(v), 'g', -1, 64)))))
		}
	case *Rational:
		r.Set(&v.v)
	default:
		panic(rbConvError(v, what))
	}
	return r
}

// rbDateSplit is r units of unit nanoseconds as whole seconds and leftover nanoseconds, floored.
func rbDateSplit(r *big.Rat, unit int64) (sec, ns int) {
	n := new(big.Rat).Mul(r, new(big.Rat).SetInt64(unit))
	q := new(big.Int).Div(n.Num(), n.Denom())
	s, rem := new(big.Int).DivMod(q, big.NewInt(1e9), new(big.Int))
	return int(s.Int64()), int(rem.Int64())
}

// rbDateSecArg is DateTime.new's seconds argument (Integer, Float or Rational).
func rbDateSecArg(v any) (sec, ns int) {
	if i, ok := rbUnbox(v).(Integer); ok {
		return int(i), 0
	}
	return rbDateSplit(rbDateRat(v, "Integer"), 1e9)
}

// rbDateAddDays is p moved by n days (Integer, Float or Rational).
func rbDateAddDays(p rbDateParts, n any) rbDateParts {
	sec, ns := p.abs()
	if i, ok := rbUnbox(n).(Integer); ok {
		return rbDateAt(sec+int(i)*86400, ns, p.of)
	}
	dsec, dns := rbDateSplit(rbDateRat(n, "Integer"), 86400e9)
	return rbDateAt(sec+dsec, ns+dns, p.of)
}

// rbDateAddMonths is p moved n months, the day clamped to the target month's end, as MRI.
func rbDateAddMonths(p rbDateParts, n int) rbDateParts {
	t := rbJDTime(p.jd)
	ms := t.Year()*12 + int(t.Month()) - 1 + n
	y, m := ms/12, ms%12+1
	if ms < 0 && ms%12 != 0 {
		y, m = (ms-11)/12, ms-((ms-11)/12)*12+1
	}
	p.jd, _ = rbCivilJD(y, m, min(t.Day(), rbMonthDays(y, m)))
	return p
}

func rbDateCmp(a, b *Date) int {
	as, an := rbDateGet(a).abs()
	bs, bn := rbDateGet(b).abs()
	if c := cmp.Compare(as, bs); c != 0 {
		return c
	}
	return cmp.Compare(an, bn)
}

// rbDateMinus is a - b in days, exactly.
func rbDateMinus(a, b *Date) *Rational {
	as, an := rbDateGet(a).abs()
	bs, bn := rbDateGet(b).abs()
	num := new(big.Int).Mul(big.NewInt(int64(as-bs)), big.NewInt(1e9))
	num.Add(num, big.NewInt(int64(an-bn)))
	out := &Rational{}
	out.v.SetFrac(num, big.NewInt(86400e9))
	return out
}

func rbDateHash(d *Date) Integer {
	s, ns := rbDateGet(d).abs()
	return Integer(s*1000003 ^ ns)
}

// rbDateZoneName is an offset as MRI's DateTime#zone prints it: "+09:00".
func rbDateZoneName(of int) string {
	sign := '+'
	if of < 0 {
		sign, of = '-', -of
	}
	return fmt.Sprintf("%c%02d:%02d", sign, of/3600, of%3600/60)
}

// rbDateGoTime is p as a Go time in a fixed zone named like its offset, so %Z prints "+09:00".
func rbDateGoTime(p rbDateParts) time.Time {
	sec, ns := p.abs()
	return time.Unix(int64(sec-rbJDEpoch*86400), int64(ns)).In(time.FixedZone(rbDateZoneName(p.of), p.of))
}

func rbDateFromGoTime(t time.Time) rbDateParts {
	_, of := t.Zone()
	return rbDateAt(int(t.Unix())+rbJDEpoch*86400, t.Nanosecond(), of)
}

// rbDateQ is a %Q directive (with rbStrftime's numeric flags and width), or a %% to skip.
var rbDateQ = regexp.MustCompile(`%%|%([-0_^#]*)(\d*)Q`)

// rbDateStrftime is Date#strftime: Time's directives plus %Q (milliseconds since the epoch).
func rbDateStrftime(p rbDateParts, format string) string {
	t := rbDateGoTime(p)
	if strings.Contains(format, "Q") {
		format = rbDateQ.ReplaceAllStringFunc(format, func(m string) string {
			if m == "%%" {
				return m
			}
			g := rbDateQ.FindStringSubmatch(m)
			s, width, pad := strconv.FormatInt(t.UnixMilli(), 10), rbAtoi(g[2]), "0"
			switch {
			case strings.Contains(g[1], "-"):
				width = 0
			case strings.Contains(g[1], "_"):
				pad = " "
			}
			return strings.Repeat(pad, max(0, width-len(s))) + s
		})
	}
	return rbStrftime(t, format, false)
}

// rbDateFrac is n digits of p's second fraction after a dot, or "" for n <= 0.
func rbDateFrac(p rbDateParts, n int) string {
	if n <= 0 {
		return ""
	}
	s := fmt.Sprintf("%09d", p.sf)
	for len(s) < n {
		s += "0"
	}
	return "." + s[:n]
}

// rbDateInspect is MRI's Date/DateTime#inspect, which shows the UTC day, seconds and nanoseconds.
func rbDateInspect(class string, p rbDateParts, shown string) string {
	sec, ns := p.abs()
	u := rbDateAt(sec, ns, 0)
	return fmt.Sprintf("#<%s: %s ((%dj,%ds,%dn),%+ds,2299161j)>", class, shown, u.jd, u.df, u.sf, p.of)
}

// rbDateZones are the zone names DateTime reads besides numeric offsets (decision 133); MRI knows more.
var rbDateZones = map[string]int{
	"UTC": 0, "UT": 0, "GMT": 0, "Z": 0,
	"EST": -5 * 3600, "EDT": -4 * 3600, "CST": -6 * 3600, "CDT": -5 * 3600,
	"MST": -7 * 3600, "MDT": -6 * 3600, "PST": -8 * 3600, "PDT": -7 * 3600,
	"AKST": -9 * 3600, "AKDT": -8 * 3600, "HST": -10 * 3600,
	"BST": 3600, "CET": 3600, "CEST": 2 * 3600, "EET": 2 * 3600, "EEST": 3 * 3600,
	"IST": 5*3600 + 1800, "JST": 9 * 3600, "KST": 9 * 3600,
	"AEST": 10 * 3600, "AEDT": 11 * 3600, "NZST": 12 * 3600, "NZDT": 13 * 3600,
}

var rbDateZoneRe = regexp.MustCompile(`^(?:(?:GMT|UTC))?([-+])(\d{1,2})(?::?(\d{2})(?::?(\d{2}))?)?$`)

// rbDateZoneOffset reads a zone: a name from rbDateZones, or ±H, ±HH, ±HHMM, ±HH:MM[:SS], optionally after GMT/UTC.
func rbDateZoneOffset(s string) (int, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if off, ok := rbDateZones[s]; ok {
		return off, true
	}
	if len(s) == 1 {
		if off, ok := rbTimeMilitaryZones[s[0]]; ok {
			return off, true
		}
	}
	g := rbDateZoneRe.FindStringSubmatch(s)
	if g == nil {
		return 0, false
	}
	off := rbAtoi(g[2])*3600 + rbAtoi(g[3])*60 + rbAtoi(g[4])
	if g[1] == "-" {
		off = -off
	}
	return off, true
}

// rbDateOffsetArg: a zone String or a fraction of a day; an unreadable or out-of-range offset is ignored (0), as MRI.
func rbDateOffsetArg(v any) int {
	var off int
	switch x := rbUnbox(v).(type) {
	case String:
		o, ok := rbDateZoneOffset(string(x))
		if !ok {
			return 0
		}
		off = o
	case nil:
		return 0
	default:
		r := rbDateRat(v, "Rational")
		r.Mul(r, big.NewRat(86400, 1))
		f, _ := r.Float64()
		off = int(math.Round(f))
	}
	if off < -86400 || off > 86400 {
		return 0
	}
	return off
}

// rbDateNow is the current instant at the local offset.
func rbDateNow() rbDateParts {
	return rbDateFromGoTime(time.Now())
}

var rbDateISO = regexp.MustCompile(`^\s*(-?\d{4,})-?(\d{2})-?(\d{2})`)
var rbDateSlash = regexp.MustCompile(`^\s*(-?\d+)/(\d{1,2})/(\d{1,2})`)
var rbDateWords = regexp.MustCompile(`(?i)^\s*(?:[a-z]+,?\s+)??(?:(\d{1,2})\s+([a-z]{3,})\.?|([a-z]{3,})\.?\s+(\d{1,2})(?:st|nd|rd|th)?,?)\s+(-?\d+)`)

func rbMonthByName(s string) int {
	s = strings.ToLower(s)
	for m := time.January; m <= time.December; m++ {
		name := strings.ToLower(m.String())
		if len(s) >= 3 && strings.HasPrefix(name, s) {
			return int(m)
		}
	}
	return 0
}

// rbDateParse covers Date.parse's common shapes: ISO (2024-03-05, 20240305), 2024/3/5, "Mar 5, 2024" and "5 March 2024".
func rbDateParse(s string) (y, m, d int, ok bool) {
	y, m, d, _, ok = rbDateParseRest(s)
	return y, m, d, ok
}

// rbDateParseRest is rbDateParse plus the text after the date.
func rbDateParseRest(s string) (y, m, d int, rest string, ok bool) {
	if g := rbDateISO.FindStringSubmatch(s); g != nil {
		return rbAtoi(g[1]), rbAtoi(g[2]), rbAtoi(g[3]), s[len(g[0]):], true
	}
	if g := rbDateSlash.FindStringSubmatch(s); g != nil {
		return rbAtoi(g[1]), rbAtoi(g[2]), rbAtoi(g[3]), s[len(g[0]):], true
	}
	if g := rbDateWords.FindStringSubmatch(s); g != nil {
		if g[1] != "" {
			m, d = rbMonthByName(g[2]), rbAtoi(g[1])
		} else {
			m, d = rbMonthByName(g[3]), rbAtoi(g[4])
		}
		return rbAtoi(g[5]), m, d, s[len(g[0]):], m != 0
	}
	return 0, 0, 0, "", false
}

var rbDateTimeRest = regexp.MustCompile(`(?i)^(?:T|\s+(?:at\s+)?)(\d{1,2}):(\d{2})(?::(\d{2})(?:[.,](\d+))?)?\s*(?:([ap])\.?m\.?)?\s*(z\b|[a-z]{2,4}\b|(?:gmt|utc)?[-+]\d{1,2}(?::?\d{2}(?::?\d{2})?)?)?`)

// rbDateFracNs is a run of fraction digits as nanoseconds.
func rbDateFracNs(digits string) int {
	if digits == "" {
		return 0
	}
	digits = (digits + "000000000")[:9]
	n, _ := strconv.Atoi(digits)
	return n
}

// rbDateTimeParse is DateTime.parse: rbDateParse's date, then an optional time of day (12- or 24-hour) and zone.
func rbDateTimeParse(s string) rbDateParts {
	y, m, d, rest, ok := rbDateParseRest(s)
	if !ok {
		panic(rbDateInvalid())
	}
	g := rbDateTimeRest.FindStringSubmatch(rest)
	if g == nil {
		return rbDateTimeOf(y, m, d, 0, 0, 0, 0, 0)
	}
	h := rbAtoi(g[1])
	switch strings.ToLower(g[5]) {
	case "p":
		h = h%12 + 12
	case "a":
		h %= 12
	}
	of, _ := rbDateZoneOffset(g[6])
	return rbDateTimeOf(y, m, d, h, rbAtoi(g[2]), rbAtoi(g[3]), rbDateFracNs(g[4]), of)
}

// rbDateTimeMatch builds parts from a regexp's groups: year, month, day, hour, minute, second, fraction, zone.
func rbDateTimeMatch(re *regexp.Regexp, s string) (rbDateParts, bool) {
	g := re.FindStringSubmatch(s)
	if g == nil {
		return rbDateParts{}, false
	}
	m := rbAtoi(g[2])
	if m == 0 {
		m = rbMonthByName(g[2])
	}
	of := 0
	if g[8] != "" {
		o, ok := rbDateZoneOffset(g[8])
		if !ok {
			return rbDateParts{}, false
		}
		of = o
	}
	return rbDateTimeOf(rbAtoi(g[1]), m, rbAtoi(g[3]), rbAtoi(g[4]), rbAtoi(g[5]), rbAtoi(g[6]), rbDateFracNs(g[7]), of), true
}

var (
	rbDateISOExt   = regexp.MustCompile(`^\s*([-+]?\d{4,})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2})(?::(\d{2})(?:[.,](\d+))?)?\s*(Z|[-+]\d{2}(?::?\d{2})?)?)?\s*$`)
	rbDateISOBasic = regexp.MustCompile(`^\s*([-+]?\d{4})(\d{2})(\d{2})(?:T(\d{2})(\d{2})(?:(\d{2})(?:[.,](\d+))?)?\s*(Z|[-+]\d{2}(?:\d{2})?)?)?\s*$`)
	rbDateRFC3339  = regexp.MustCompile(`^\s*(-?\d{4})-(\d{2})-(\d{2})[Tt ](\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?([Zz]|[-+]\d{2}:\d{2})\s*$`)
	rbDateHTTP     = regexp.MustCompile(`^\s*[A-Za-z]{3},\s+(\d{2})\s+([A-Za-z]{3})\s+(\d{4})\s+(\d{2}):(\d{2}):(\d{2})()\s+(GMT)\s*$`)
	rbDateRFC2822  = regexp.MustCompile(`^\s*(?:[A-Za-z]{3},\s*)?(\d{1,2})\s+([A-Za-z]{3})\s+(\d{2,})\s+(\d{2}):(\d{2})(?::(\d{2}))?()\s+([-+]\d{4}|[A-Za-z]{1,4})\s*$`)
	rbDateJISTime  = regexp.MustCompile(`^\s*([MTSHR])?(\d{2})\.(\d{2})\.(\d{2})(?:T(\d{2}):(\d{2})(?::(\d{2})(?:[.,](\d+))?)?\s*(Z|[-+]\d{2}(?::?\d{2})?)?)?\s*$`)
)

// rbDateTimeISO8601 is DateTime.iso8601/xmlschema: extended or basic calendar dates, with an optional time and zone.
func rbDateTimeISO8601(s string) rbDateParts {
	for _, re := range []*regexp.Regexp{rbDateISOExt, rbDateISOBasic} {
		if p, ok := rbDateTimeMatch(re, s); ok {
			return p
		}
	}
	panic(rbDateInvalid())
}

// rbDateTimeFormat parses s with one of the fixed-shape regexps (rbDateRFC3339, rbDateHTTP, rbDateRFC2822).
func rbDateTimeFormat(s string, re *regexp.Regexp) rbDateParts {
	g := re.FindStringSubmatch(s)
	if g == nil {
		panic(rbDateInvalid())
	}
	if re != rbDateRFC3339 {
		// day comes first in httpdate and rfc2822: reorder to y, m, d
		g[1], g[3] = g[3], g[1]
		if y, _ := strconv.Atoi(g[1]); len(g[1]) < 4 {
			switch {
			case y < 50:
				g[1] = strconv.Itoa(y + 2000)
			case y < 1000:
				g[1] = strconv.Itoa(y + 1900)
			}
		}
	}
	m := rbMonthByName(g[2])
	if m == 0 {
		m = rbAtoi(g[2])
	}
	of, ok := rbDateZoneOffset(g[8])
	if !ok {
		panic(rbDateInvalid())
	}
	return rbDateTimeOf(rbAtoi(g[1]), m, rbAtoi(g[3]), rbAtoi(g[4]), rbAtoi(g[5]), rbAtoi(g[6]), rbDateFracNs(g[7]), of)
}

// rbDateTimeJISX0301 is DateTime.jisx0301: an era date with an optional time, or ISO 8601.
func rbDateTimeJISX0301(s string) rbDateParts {
	g := rbDateJISTime.FindStringSubmatch(s)
	if g == nil {
		return rbDateTimeISO8601(s)
	}
	y, ok := rbJISEraYear(g[1], rbAtoi(g[2]))
	if !ok {
		panic(rbDateInvalid())
	}
	of := 0
	if g[9] != "" {
		of, _ = rbDateZoneOffset(g[9])
	}
	return rbDateTimeOf(y, rbAtoi(g[3]), rbAtoi(g[4]), rbAtoi(g[5]), rbAtoi(g[6]), rbAtoi(g[7]), rbDateFracNs(g[8]), of)
}

// rbDateStrptime reads the date fields of MRI's strptime: %Y %m %d %e %y %j %b %B %h %F %D %%.
func rbDateStrptime(s, format string) (y, m, d int, ok bool) {
	p, ok := rbDateStrptimeAll(s, format)
	return p.y, p.m, p.d, ok
}

// rbDateFields is what strptime read.
type rbDateFields struct {
	y, m, d, h, mi, s, ns, of int
	zone                      string // the %z/%Z text as given, "" when the format had none (Time.strptime: local time)
	epoch                     bool   // from %s/%Q: the fields are UTC
}

// rbDateStrptimeAll is MRI's strptime over the date directives above plus
// %H %k %I %l %M %S %L %N %p %P %z %Z %s %Q %a %A and the composites %T %R %X %r %c %x.
func rbDateStrptimeAll(s, format string) (rbDateFields, bool) {
	// "%%" maps to itself so an escaped "%%T" is not expanded as %T.
	format = strings.NewReplacer("%F", "%Y-%m-%d", "%D", "%m/%d/%y", "%x", "%m/%d/%y", "%T", "%H:%M:%S", "%X", "%H:%M:%S",
		"%R", "%H:%M", "%r", "%I:%M:%S %p", "%c", "%a %b %e %H:%M:%S %Y", "%+", "%a %b %e %H:%M:%S %Z %Y", "%%", "%%").Replace(format)
	f := rbDateFields{y: -4712, m: 1, d: 1}
	yday, pm, ampm, epoch, hasEpoch := 0, false, false, 0, false
	num := func(max int) (int, bool) {
		i := 0
		if i < len(s) && (s[i] == '-' || s[i] == '+') && max > 2 {
			i++
		}
		for i < len(s) && i < max+1 && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		n, err := strconv.Atoi(s[:i])
		s = s[i:]
		return n, err == nil
	}
	word := func() string {
		j := 0
		for j < len(s) && (s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
			j++
		}
		w := s[:j]
		s = s[j:]
		return w
	}
	fail := rbDateFields{}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c == ' ' {
			s = strings.TrimLeft(s, " \t\n")
			continue
		}
		if c != '%' || i+1 >= len(format) {
			if s == "" || s[0] != c {
				return fail, false
			}
			s = s[1:]
			continue
		}
		i++
		for i+1 < len(format) && format[i] == ':' {
			i++
		}
		var good bool
		switch format[i] {
		case 'Y':
			f.y, good = num(9)
		case 'm':
			f.m, good = num(2)
		case 'd', 'e':
			s = strings.TrimLeft(s, " ")
			f.d, good = num(2)
		case 'y':
			f.y, good = num(2)
			if f.y < 69 {
				f.y += 2000
			} else {
				f.y += 1900
			}
		case 'j':
			yday, good = num(3)
		case 'b', 'B', 'h':
			f.m = rbMonthByName(word())
			good = f.m != 0
		case 'a', 'A':
			good = len(word()) >= 3
		case 'H', 'k', 'I', 'l':
			s = strings.TrimLeft(s, " ")
			f.h, good = num(2)
		case 'M':
			f.mi, good = num(2)
		case 'S':
			f.s, good = num(2)
		case 'L', 'N':
			j := 0
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			f.ns, good, s = rbDateFracNs(s[:j]), j > 0, s[j:]
		case 'p', 'P':
			w := strings.ToLower(strings.ReplaceAll(s[:min(len(s), 4)], ".", ""))
			good = strings.HasPrefix(w, "am") || strings.HasPrefix(w, "pm")
			pm, ampm = strings.HasPrefix(w, "pm"), true
			if good && len(s) >= 4 && s[1] == '.' {
				s = s[4:]
			} else if good {
				s = s[2:]
			}
		case 'z', 'Z':
			j := 0
			for j < len(s) && strings.IndexByte("+-:0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", s[j]) >= 0 {
				j++
			}
			f.of, good = rbDateZoneOffset(s[:j])
			f.zone = s[:j]
			s = s[j:]
		case 's', 'Q':
			epoch, good = num(19)
			hasEpoch = true
			if format[i] == 's' {
				epoch *= 1000
			}
		case '%':
			good = s != "" && s[0] == '%'
			if good {
				s = s[1:]
			}
		}
		if !good {
			return fail, false
		}
	}
	if ampm {
		f.h %= 12
		if pm {
			f.h += 12
		}
	}
	if hasEpoch {
		p := rbDateAt(epoch/1000+rbJDEpoch*86400, epoch%1000*1e6, f.of)
		t := rbJDTime(p.jd)
		return rbDateFields{y: t.Year(), m: int(t.Month()), d: t.Day(), h: p.df / 3600, mi: p.df % 3600 / 60, s: p.df % 60, ns: p.sf, of: f.of, zone: f.zone, epoch: true}, true
	}
	if yday > 0 {
		t := time.Date(f.y, 1, yday, 0, 0, 0, 0, time.UTC)
		if t.Year() != f.y {
			return fail, false
		}
		f.m, f.d = int(t.Month()), t.Day()
	}
	return f, true
}

// rbDateTimeStrptime is DateTime.strptime.
func rbDateTimeStrptime(s, format string) rbDateParts {
	f, ok := rbDateStrptimeAll(s, format)
	if !ok {
		panic(rbDateInvalid())
	}
	return rbDateTimeOf(f.y, f.m, f.d, f.h, f.mi, f.s, f.ns, f.of)
}

// rbEras are the JIS X 0301 Japanese era codes, most recent first, each with its first Gregorian day and the year it calls 1.
var rbEras = []struct {
	y, m, d  int
	code     byte
	yearBase int
}{
	{2019, 5, 1, 'R', 2019},
	{1989, 1, 8, 'H', 1989},
	{1926, 12, 25, 'S', 1926},
	{1912, 7, 30, 'T', 1912},
	{1873, 1, 1, 'M', 1868}, // Japan adopted the Gregorian calendar in Meiji 6; MRI's jisx0301 has no Meiji dates before it
}

// rbJISX0301 is jd's JIS X 0301 date, or plain ISO before the Meiji era (as MRI's).
func rbJISX0301(jd int) string {
	t := rbJDTime(jd)
	for _, e := range rbEras {
		if start, ok := rbCivilJD(e.y, e.m, e.d); ok && jd >= start {
			return fmt.Sprintf("%c%02d.%02d.%02d", e.code, t.Year()-e.yearBase+1, int(t.Month()), t.Day())
		}
	}
	return rbStrftime(t, "%Y-%m-%d", true)
}

var rbJISX0301Re = regexp.MustCompile(`^([MTSHR])?(\d{2})\.(\d{2})\.(\d{2})$`)

// rbJISEraYear is the Gregorian year of year n of era code; no code is Heisei, as MRI.
func rbJISEraYear(code string, n int) (int, bool) {
	c := byte('H')
	if code != "" {
		c = code[0]
	}
	for _, e := range rbEras {
		if e.code == c {
			return e.yearBase + n - 1, true
		}
	}
	return 0, false
}

// rbJISX0301Parse reads a JIS X 0301 date, or falls back to rbDateParse's ISO shape.
func rbJISX0301Parse(s string) (y, m, d int, ok bool) {
	g := rbJISX0301Re.FindStringSubmatch(s)
	if g == nil {
		return rbDateParse(s)
	}
	y, ok = rbJISEraYear(g[1], rbAtoi(g[2]))
	return y, rbAtoi(g[3]), rbAtoi(g[4]), ok
}

func (x *DateTime) rbSuccAny() any { return x.Succ_ofDateTime() }
