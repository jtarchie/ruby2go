// Hand-written Go port of benchmarks/format.rb, for the README comparison column.
package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

func rubyFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func rjust(s string, w int, pad string) string {
	if n := w - len(s); n > 0 {
		return strings.Repeat(pad, n) + s
	}
	return s
}

func ljust(s string, w int, pad string) string {
	if n := w - len(s); n > 0 {
		return s + strings.Repeat(pad, n)
	}
	return s
}

// center puts the odd pad character on the right, as Ruby does.
func center(s string, w int, pad string) string {
	n := w - len(s)
	if n <= 0 {
		return s
	}
	return strings.Repeat(pad, n/2) + s + strings.Repeat(pad, n-n/2)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func reverse(s string) string {
	b := []byte(s) // ASCII input
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

func main() {
	var out strings.Builder
	for i := range 100_000 {
		fmt.Fprintf(&out, "%05d|%-8s|%8.3f|%x\n", i, "n"+strconv.Itoa(i%97), float64(i)/7.0, i)
	}
	s := out.String()
	fmt.Println(len(s))
	fmt.Print(strings.SplitAfter(s, "\n")[12_345])

	rows := make([]string, 50_000)
	for i := range rows {
		rows[i] = rjust(strconv.FormatInt(int64(i), 2), 20, "0") + " " + ljust(rubyFloat(float64(i)*1.5), 10, " ") + center(strconv.Itoa(i), 9, "*")
	}
	buf := strings.Join(rows, "\n")
	fmt.Println(utf8.RuneCountInString(buf))
	lines := strings.SplitAfter(buf, "\n")
	fmt.Println(lines[len(lines)-1])

	words := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	acc := 0
	for i := range 200_000 {
		w := words[i%5]
		acc += len(strings.ToUpper(w) + "-" + capitalize(w) + "-" + reverse(w) + ":" + strconv.Itoa(i))
	}
	fmt.Println(acc)

	fs := make([]string, 0, 100_000)
	for i := 1; i <= 100_000; i++ {
		fs = append(fs, strconv.FormatFloat(float64(i)/3.0, 'f', 2, 64))
	}
	fmt.Println(len(strings.Join(fs, ",")))

	total := 0
	for i := 1; i <= 100_000; i++ {
		for _, c := range []byte(strconv.Itoa(i)) {
			total += int(c)
		}
		for n := i; n > 0; n /= 10 {
			total += n % 10
		}
	}
	fmt.Println(total)
}
