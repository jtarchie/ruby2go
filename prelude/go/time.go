//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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

// rbTimeToS is Time#to_s: MRI prints UTC as "UTC" and any other zone as its offset.
func rbTimeToS(t time.Time, utc bool) string {
	if utc {
		return rbStrftime(t, "%Y-%m-%d %H:%M:%S UTC", utc)
	}
	return rbStrftime(t, "%Y-%m-%d %H:%M:%S %z", utc)
}

func rbTimeInspect(t time.Time, utc bool) string {
	s := rbStrftime(t, "%Y-%m-%d %H:%M:%S", utc)
	if ns := t.Nanosecond(); ns != 0 {
		s += "." + strings.TrimRight(fmt.Sprintf("%09d", ns), "0")
	}
	if utc {
		return s + " UTC"
	}
	return s + " " + rbStrftime(t, "%z", utc)
}

// rbStrftime is MRI's strftime: flags - 0 _ ^ #, a width, and : for %z.
func rbStrftime(t time.Time, format string, utc bool) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 >= len(format) {
			b.WriteByte(c)
			continue
		}
		start := i
		i++
		var flags []byte
		for i < len(format) && strings.IndexByte("-0_^#:", format[i]) >= 0 {
			flags = append(flags, format[i])
			i++
		}
		width := -1
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			if width < 0 {
				width = 0
			}
			width = width*10 + int(format[i]-'0')
			i++
		}
		if i >= len(format) {
			b.WriteString(format[start:])
			break
		}
		has := func(f byte) bool { return bytes.IndexByte(flags, f) >= 0 }
		num := func(v, w int, pad byte) string {
			if width >= 0 {
				w = width
			}
			switch {
			case has('-'):
				w = 0
			case has('_'):
				pad = ' '
			case has('0'):
				pad = '0'
			}
			s := strconv.Itoa(v)
			neg := v < 0
			if neg {
				s = s[1:]
				w--
			}
			for len(s) < w {
				s = string(pad) + s
			}
			if neg {
				s = "-" + s
			}
			return s
		}
		text := func(s string) string {
			if has('^') {
				s = strings.ToUpper(s)
			}
			if has('#') {
				if strings.ToUpper(s) == s {
					s = strings.ToLower(s)
				} else {
					s = strings.ToUpper(s)
				}
			}
			pad := " "
			if has('0') {
				pad = "0"
			}
			for width > 0 && !has('-') && len(s) < width {
				s = pad + s
			}
			return s
		}
		hour12 := t.Hour() % 12
		if hour12 == 0 {
			hour12 = 12
		}
		switch format[i] {
		case 'Y':
			w := 0
			if t.Year() < 0 {
				w = 5
			} else if t.Year() < 1000 {
				w = 4
			}
			b.WriteString(num(t.Year(), w, '0'))
		case 'C':
			b.WriteString(num(t.Year()/100, 2, '0'))
		case 'y':
			b.WriteString(num(((t.Year()%100)+100)%100, 2, '0'))
		case 'm':
			b.WriteString(num(int(t.Month()), 2, '0'))
		case 'd':
			b.WriteString(num(t.Day(), 2, '0'))
		case 'e':
			b.WriteString(num(t.Day(), 2, ' '))
		case 'j':
			b.WriteString(num(t.YearDay(), 3, '0'))
		case 'H':
			b.WriteString(num(t.Hour(), 2, '0'))
		case 'k':
			b.WriteString(num(t.Hour(), 2, ' '))
		case 'I':
			b.WriteString(num(hour12, 2, '0'))
		case 'l':
			b.WriteString(num(hour12, 2, ' '))
		case 'M':
			b.WriteString(num(t.Minute(), 2, '0'))
		case 'S':
			b.WriteString(num(t.Second(), 2, '0'))
		case 'L', 'N':
			digits := 3
			if format[i] == 'N' {
				digits = 9
			}
			if width > 0 {
				digits = width
			}
			s := fmt.Sprintf("%09d", t.Nanosecond())
			for len(s) < digits {
				s += "0"
			}
			b.WriteString(s[:digits])
		case 'z':
			_, off := t.Zone()
			sign := '+'
			if off < 0 {
				sign, off = '-', -off
			}
			sep := ""
			if has(':') {
				sep = ":"
			}
			fmt.Fprintf(&b, "%c%02d%s%02d", sign, off/3600, sep, off%3600/60)
		case 'Z':
			name, _ := t.Zone()
			if utc {
				name = "UTC"
			}
			b.WriteString(text(name))
		case 'A':
			b.WriteString(text(t.Weekday().String()))
		case 'a':
			b.WriteString(text(t.Weekday().String()[:3]))
		case 'B':
			b.WriteString(text(t.Month().String()))
		case 'b', 'h':
			b.WriteString(text(t.Month().String()[:3]))
		case 'p':
			ampm := "AM"
			if t.Hour() >= 12 {
				ampm = "PM"
			}
			if has('#') {
				ampm = strings.ToLower(ampm)
				flags = nil
			}
			b.WriteString(text(ampm))
		case 'P':
			ampm := "am"
			if t.Hour() >= 12 {
				ampm = "pm"
			}
			b.WriteString(text(ampm))
		case 'u':
			wd := int(t.Weekday())
			if wd == 0 {
				wd = 7
			}
			b.WriteString(num(wd, 1, '0'))
		case 'w':
			b.WriteString(num(int(t.Weekday()), 1, '0'))
		case 'U':
			b.WriteString(num((t.YearDay()+6-int(t.Weekday()))/7, 2, '0'))
		case 'W':
			b.WriteString(num((t.YearDay()+6-(int(t.Weekday())+6)%7)/7, 2, '0'))
		case 'G':
			y, _ := t.ISOWeek()
			b.WriteString(num(y, 4, '0'))
		case 'g':
			y, _ := t.ISOWeek()
			b.WriteString(num(y%100, 2, '0'))
		case 'V':
			_, w := t.ISOWeek()
			b.WriteString(num(w, 2, '0'))
		case 's':
			b.WriteString(num(int(t.Unix()), 1, '0'))
		case 'n':
			b.WriteString("\n")
		case 't':
			b.WriteString("\t")
		case '%':
			b.WriteString("%")
		case 'F':
			b.WriteString(text(rbStrftime(t, "%Y-%m-%d", utc)))
		case 'T', 'X':
			b.WriteString(text(rbStrftime(t, "%H:%M:%S", utc)))
		case 'D', 'x':
			b.WriteString(text(rbStrftime(t, "%m/%d/%y", utc)))
		case 'R':
			b.WriteString(text(rbStrftime(t, "%H:%M", utc)))
		case 'r':
			b.WriteString(text(rbStrftime(t, "%I:%M:%S %p", utc)))
		case 'c':
			b.WriteString(text(rbStrftime(t, "%a %b %e %H:%M:%S %Y", utc)))
		case 'v':
			b.WriteString(text(rbStrftime(t, "%e-%^b-%Y", utc)))
		case '+':
			b.WriteString(text(rbStrftime(t, "%a %b %e %H:%M:%S %Z %Y", utc)))
		default:
			b.WriteString(format[start : i+1])
		}
	}
	return b.String()
}
