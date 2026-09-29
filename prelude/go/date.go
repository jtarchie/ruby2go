//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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
	atoi := func(x string) int { n, _ := strconv.Atoi(x); return n }
	if g := rbDateISO.FindStringSubmatch(s); g != nil {
		return atoi(g[1]), atoi(g[2]), atoi(g[3]), true
	}
	if g := rbDateSlash.FindStringSubmatch(s); g != nil {
		return atoi(g[1]), atoi(g[2]), atoi(g[3]), true
	}
	if g := rbDateWords.FindStringSubmatch(s); g != nil {
		if g[1] != "" {
			m, d = rbMonthByName(g[2]), atoi(g[1])
		} else {
			m, d = rbMonthByName(g[3]), atoi(g[4])
		}
		return atoi(g[5]), m, d, m != 0
	}
	return 0, 0, 0, false
}

// rbDateStrptime reads the date fields of MRI's strptime: %Y %m %d %e %y %j %b %B %h %F %D %%.
func rbDateStrptime(s, format string) (y, m, d int, ok bool) {
	format = strings.NewReplacer("%F", "%Y-%m-%d", "%D", "%m/%d/%y", "%x", "%m/%d/%y").Replace(format)
	y, m, d = -4712, 1, 1
	yday := 0
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
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c == ' ' {
			s = strings.TrimLeft(s, " \t\n")
			continue
		}
		if c != '%' || i+1 >= len(format) {
			if s == "" || s[0] != c {
				return 0, 0, 0, false
			}
			s = s[1:]
			continue
		}
		i++
		var good bool
		switch format[i] {
		case 'Y':
			y, good = num(9)
		case 'm':
			m, good = num(2)
		case 'd', 'e':
			s = strings.TrimLeft(s, " ")
			d, good = num(2)
		case 'y':
			y, good = num(2)
			if y < 69 {
				y += 2000
			} else {
				y += 1900
			}
		case 'j':
			yday, good = num(3)
		case 'b', 'B', 'h':
			j := 0
			for j < len(s) && (s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
				j++
			}
			m, good = rbMonthByName(s[:j]), true
			good = good && m != 0
			s = s[j:]
		case '%':
			good = s != "" && s[0] == '%'
			if good {
				s = s[1:]
			}
		}
		if !good {
			return 0, 0, 0, false
		}
	}
	if yday > 0 {
		t := time.Date(y, 1, yday, 0, 0, 0, 0, time.UTC)
		return t.Year(), int(t.Month()), t.Day(), t.Year() == y
	}
	return y, m, d, true
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

var rbJISX0301Re = regexp.MustCompile(`^([MTSHR])(\d{2})\.(\d{2})\.(\d{2})$`)

// rbJISX0301Parse reads a JIS X 0301 date, or falls back to rbDateParse's ISO shape.
func rbJISX0301Parse(s string) (y, m, d int, ok bool) {
	g := rbJISX0301Re.FindStringSubmatch(s)
	if g == nil {
		return rbDateParse(s)
	}
	for _, e := range rbEras {
		if e.code != g[1][0] {
			continue
		}
		yy, _ := strconv.Atoi(g[2])
		mm, _ := strconv.Atoi(g[3])
		dd, _ := strconv.Atoi(g[4])
		return e.yearBase + yy - 1, mm, dd, true
	}
	return 0, 0, 0, false
}
