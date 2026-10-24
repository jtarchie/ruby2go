// Hand-written Go port of benchmarks/regexp.rb, for the README comparison column.
// StringScanner is a small struct over ^-anchored stdlib regexps; tr/delete/squeeze are byte loops.
package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const line = "2026-10-20 12:34:56 GET /api/v1/users/42?x=1 200 1532ms user=alice@example.com"

var (
	reReq   = regexp.MustCompile(` (GET|POST) (\S+) (\d{3}) (\d+)ms`)
	reEmail = regexp.MustCompile(`\w+@\w+\.com`)
	reDigit = regexp.MustCompile(`\d`)
	reVowel = regexp.MustCompile(`[aeiou]`)
	reSplit = regexp.MustCompile(`[\s/?=]+`)
	reUsers = regexp.MustCompile(`users/1\d*\?`)
	reSpace = regexp.MustCompile(`^\s+`)
	reNum   = regexp.MustCompile(`^\d+`)
	reIdent = regexp.MustCompile(`^[a-z_]\w*`)
	rePunct = regexp.MustCompile(`^[(),;=+*]`)
)

// rot1 is tr("a-z", "b-za").
func rot1(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = 'a' + (c-'a'+1)%26
		}
	}
	return string(b)
}

// deleteDigits is delete("0-9").
func deleteDigits(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			b = append(b, s[i])
		}
	}
	return string(b)
}

// squeeze collapses runs of the same byte.
func squeeze(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if i == 0 || s[i] != s[i-1] {
			b = append(b, s[i])
		}
	}
	return string(b)
}

type scanner struct {
	src string
	pos int
}

func (s *scanner) eos() bool { return s.pos >= len(s.src) }

func (s *scanner) scan(re *regexp.Regexp) (string, bool) {
	loc := re.FindStringIndex(s.src[s.pos:])
	if loc == nil {
		return "", false
	}
	m := s.src[s.pos : s.pos+loc[1]]
	s.pos += loc[1]
	return m, true
}

func main() {
	lines := make([]string, 20_000)
	for i := range lines {
		l := strings.Replace(line, "42", strconv.Itoa(i), 1)
		lines[i] = strings.Replace(l, "1532", strconv.Itoa(i%2000), 1)
	}

	status := map[string]int{}
	var statusOrder []string
	ms := 0
	for _, l := range lines {
		if m := reReq.FindStringSubmatch(l); m != nil {
			if _, ok := status[m[3]]; !ok {
				statusOrder = append(statusOrder, m[3])
			}
			status[m[3]]++
			n, _ := strconv.Atoi(m[4])
			ms += n
		}
	}
	parts := make([]string, len(statusOrder))
	for i, k := range statusOrder {
		parts[i] = strconv.Quote(k) + " => " + strconv.Itoa(status[k])
	}
	fmt.Println("{" + strings.Join(parts, ", ") + "}")
	fmt.Println(ms)

	emails := 0
	for _, l := range lines {
		emails += len(reEmail.FindAllStringIndex(l, -1))
	}
	fmt.Println(emails)

	masked := make([]string, len(lines))
	for i, l := range lines {
		masked[i] = reDigit.ReplaceAllStringFunc(l, func(d string) string {
			return strconv.Itoa((int(d[0]-'0') + 1) % 10)
		})
	}
	fmt.Println(masked[len(masked)-1])

	vowels := make([]string, len(lines))
	for i, l := range lines {
		vowels[i] = reVowel.ReplaceAllStringFunc(l, func(v string) string {
			return string("43_10"[strings.Index("aeiou", v)])
		})
	}
	fmt.Println(vowels[0])

	fields := 0
	for _, l := range lines {
		fields += len(reSplit.Split(l, -1))
	}
	fmt.Println(fields)

	users := 0
	for _, l := range lines {
		if reUsers.MatchString(l) {
			users++
		}
	}
	fmt.Println(users)

	mapped := make([]string, len(lines))
	for i, l := range lines {
		mapped[i] = squeeze(deleteDigits(rot1(l)))
	}
	uniq := map[string]struct{}{}
	for _, s := range mapped {
		uniq[s] = struct{}{}
	}
	fmt.Println(len(uniq))

	tokens, sums := 0, 0
	ss := &scanner{src: strings.Repeat("let x = 10 + foo(20, 30) * bar; ", 4000)}
	for !ss.eos() {
		if _, ok := ss.scan(reSpace); ok {
			continue
		}
		if n, ok := ss.scan(reNum); ok {
			v, _ := strconv.Atoi(n)
			sums += v
		} else if _, ok := ss.scan(reIdent); ok {
			tokens++
		} else if _, ok := ss.scan(rePunct); ok {
			tokens++
		} else {
			ss.pos++ // getch: ASCII source
		}
	}
	fmt.Println(tokens)
	fmt.Println(sums)
}
