//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

func rbTimeArgs(y, mo, d, h, mi, s Integer) {
	var msg string
	switch {
	case mo < 1 || mo > 12:
		msg = "mon out of range"
	case d < 1 || d > 31:
		msg = "argument out of range"
	case h < 0 || h > 24:
		msg = "hour out of range"
	case mi < 0 || mi > 59:
		msg = "min out of range"
	case s < 0 || s > 60:
		msg = "sec out of range"
	default:
		return
	}
	panic(NewArgumentError(Ref(String(msg))))
}

func rbTimeFromFloat(f float64) time.Time {
	sec := math.Floor(f)
	return time.Unix(int64(sec), int64(math.Round((f-sec)*1e9)))
}

// rbTimeZoneArg resolves nil/Integer-offset/zone-String/`in:`-Hash (decision 23 has no real keyword params) to a Location and MRI's UTC flag (decision 39).
// ponytail: no TypeError for a wrong argument class (e.g. Time.at's unsupported subsec arg); only unrecognised zone strings/out-of-range offsets raise.
func rbTimeZoneArg(zone any, def *time.Location) (*time.Location, bool) {
	switch v := zone.(type) {
	case String:
		return rbTimeZoneLocation(rbTimeZoneOffset(string(v)))
	case Integer:
		return rbTimeZoneLocation(rbTimeZoneOffsetInt(int(v)))
	case *Hash[Symbol, String]:
		if z, ok := v.vals[Symbol("in")]; ok {
			return rbTimeZoneLocation(rbTimeZoneOffset(string(z)))
		}
	}
	return def, false
}

func rbTimeZoneLocation(off int, isUTC bool) (*time.Location, bool) {
	if isUTC {
		return time.UTC, true
	}
	return time.FixedZone("", off), false
}

func rbTimeZoneOffsetInt(n int) (off int, isUTC bool) {
	if n <= -86400 || n >= 86400 {
		panic(NewArgumentError(Ref(String("utc_offset out of range"))))
	}
	return n, false
}

// rbTimeMilitaryZones maps MRI's single-letter zones (A-I,K-Y; J is unused, Z is handled as UTC alongside the word "UTC").
var rbTimeMilitaryZones = map[byte]int{
	'A': 3600, 'B': 7200, 'C': 10800, 'D': 14400, 'E': 18000, 'F': 21600,
	'G': 25200, 'H': 28800, 'I': 32400, 'K': 36000, 'L': 39600, 'M': 43200,
	'N': -3600, 'O': -7200, 'P': -10800, 'Q': -14400, 'R': -18000, 'S': -21600,
	'T': -25200, 'U': -28800, 'V': -32400, 'W': -36000, 'X': -39600, 'Y': -43200,
}

// rbTimeZoneOffset parses a zone String as MRI's Time.new/localtime do.
func rbTimeZoneOffset(zone string) (off int, isUTC bool) {
	if strings.EqualFold(zone, "UTC") || zone == "Z" {
		return 0, true
	}
	if len(zone) == 1 {
		if off, ok := rbTimeMilitaryZones[zone[0]]; ok {
			return off, false
		}
		panic(rbTimeZoneFormatError(zone))
	}
	if zone[0] != '+' && zone[0] != '-' {
		panic(rbTimeZoneFormatError(zone))
	}
	hh, mm, ss, ok := rbTimeZoneDigits(zone[1:])
	if !ok {
		panic(rbTimeZoneFormatError(zone))
	}
	total := hh*3600 + mm*60 + ss
	if zone[0] == '-' && total == 0 {
		return 0, true // "-00:00"/"-0000": MRI treats a negative zero offset as UTC.
	}
	if zone[0] == '-' {
		total = -total
	}
	return rbTimeZoneOffsetInt(total)
}

// rbTimeZoneDigits reads HH, HH:MM, HH:MM:SS, HHMM or HHMMSS after the sign.
func rbTimeZoneDigits(s string) (hh, mm, ss int, ok bool) {
	d2 := func(s string, max int) (int, bool) {
		if len(s) != 2 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
			return 0, false
		}
		v := int(s[0]-'0')*10 + int(s[1]-'0')
		return v, v <= max
	}
	switch len(s) {
	case 2:
		h, hok := d2(s, 99)
		return h, 0, 0, hok
	case 4:
		h, hok := d2(s[0:2], 99)
		m, mok := d2(s[2:4], 59)
		return h, m, 0, hok && mok
	case 5:
		if s[2] != ':' {
			return 0, 0, 0, false
		}
		h, hok := d2(s[0:2], 99)
		m, mok := d2(s[3:5], 59)
		return h, m, 0, hok && mok
	case 6:
		h, hok := d2(s[0:2], 99)
		m, mok := d2(s[2:4], 59)
		sec, sok := d2(s[4:6], 59)
		return h, m, sec, hok && mok && sok
	case 8:
		if s[2] != ':' || s[5] != ':' {
			return 0, 0, 0, false
		}
		h, hok := d2(s[0:2], 99)
		m, mok := d2(s[3:5], 59)
		sec, sok := d2(s[6:8], 59)
		return h, m, sec, hok && mok && sok
	}
	return 0, 0, 0, false
}

func rbTimeZoneFormatError(zone string) any {
	return NewArgumentError(Ref(String(`"+HH:MM", "-HH:MM", "UTC" or "A".."I","K".."Z" expected for utc_offset: ` + zone)))
}

// rbTimeRound rounds/floors/ceils (mode -1/0/1) t's fractional second to `digits` decimal places, ties toward +infinity as MRI's Time#round does (unlike Float#round's ties-away-from-zero).
func rbTimeRound(t time.Time, digits, mode int) time.Time {
	if digits < 0 {
		panic(NewArgumentError(Ref(String("negative ndigits given"))))
	}
	if digits > 9 {
		digits = 9
	}
	unit := int64(1)
	for range 9 - digits {
		unit *= 10
	}
	nsec := int64(t.Nanosecond())
	var rounded int64
	switch {
	case mode < 0:
		rounded = (nsec / unit) * unit
	case mode > 0:
		rounded = ((nsec + unit - 1) / unit) * unit
	default:
		rounded = ((nsec + unit/2) / unit) * unit
	}
	sec := t.Unix()
	if rounded >= 1_000_000_000 {
		sec++
		rounded = 0
	}
	return time.Unix(sec, rounded).In(t.Location())
}

// rbTimeToS is Time#to_s: MRI prints UTC as "UTC" and any other zone as its offset.
func rbTimeToS(t time.Time, utc bool) string {
	if utc {
		return t.Format("2006-01-02 15:04:05") + " UTC"
	}
	return t.Format("2006-01-02 15:04:05 -0700")
}

func rbTimeInspect(t time.Time, utc bool) string {
	s := t.Format("2006-01-02 15:04:05")
	if ns := t.Nanosecond(); ns != 0 {
		s += "." + strings.TrimRight(fmt.Sprintf("%09d", ns), "0")
	}
	if utc {
		return s + " UTC"
	}
	return s + " " + t.Format("-0700")
}

var (
	rbTimeISOLayouts     = []string{"2006-01-02T15:04:05.999999999Z07:00", "2006-01-02T15:04:05.999999999", "2006-01-02"}
	rbTimeRFC2822Layouts = []string{"Mon, 2 Jan 2006 15:04:05 -0700", "2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST", "2 Jan 2006 15:04:05 MST"}
	// ponytail: Time.parse's shapes are a fixed list; Date._parse's free-form heuristics would replace it.
	rbTimeLayouts = slices.Concat(rbTimeISOLayouts, rbTimeRFC2822Layouts, []string{
		"2006-01-02 15:04:05.999999999 -0700", "2006-01-02 15:04:05.999999999 -07:00", "2006-01-02 15:04:05.999999999 MST",
		"2006-01-02 15:04:05.999999999", "2006-01-02 15:04", "2006/01/02 15:04:05", "2006/01/02",
		time.ANSIC, time.UnixDate, "Jan 2 2006 15:04:05", "Jan 2 2006", "January 2, 2006", "2 January 2006",
	})
)

// rbTimeParseOr parses s with the first layout that fits, in local time
// unless s carries a zone; a Z, UTC or GMT zone is MRI's UTC mode.
func rbTimeParseOr(s string, layouts []string, msg string) *Time {
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, strings.TrimSpace(s), time.Local); err == nil {
			if name, off := t.Zone(); off == 0 && (name == "UTC" || name == "GMT") && t.Location() != time.Local {
				return &Time{t: t.UTC(), utc: true}
			}
			return &Time{t: t}
		}
	}
	panic(NewArgumentError(Ref(String(msg))))
}

// rbTimeStrptime is Time.strptime over Date's strptime engine (rbDateStrptimeAll, every MRI directive): local time
// unless the format read a zone, UTC mode for Z/UTC/GMT, a fixed offset otherwise; an epoch is local time too.
func rbTimeStrptime(s, format string) *Time {
	f, ok := rbDateStrptimeAll(s, format)
	if !ok {
		panic(NewArgumentError(Ref(String("invalid date or strptime format - '" + s + "' '" + format + "'"))))
	}
	switch z := strings.ToUpper(f.zone); {
	case f.epoch:
		return &Time{t: time.Date(f.y, time.Month(f.m), f.d, f.h, f.mi, f.s, f.ns, time.UTC).In(time.Local)}
	case z == "":
		return &Time{t: time.Date(f.y, time.Month(f.m), f.d, f.h, f.mi, f.s, f.ns, time.Local)}
	case z == "Z" || z == "UTC" || z == "GMT":
		return &Time{t: time.Date(f.y, time.Month(f.m), f.d, f.h, f.mi, f.s, f.ns, time.UTC), utc: true}
	}
	return &Time{t: time.Date(f.y, time.Month(f.m), f.d, f.h, f.mi, f.s, f.ns, time.FixedZone("", f.of))}
}
