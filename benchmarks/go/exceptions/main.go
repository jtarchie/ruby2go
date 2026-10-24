// Hand-written Go port of benchmarks/exceptions.rb, for the README comparison column.
// raise/rescue map to error values; catch/throw to an early return; the /\A\d\z/ check is a byte test.
package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
)

type ParseError struct {
	Msg string
	Pos int
}

func (e *ParseError) Error() string { return e.Msg }

func parseDigit(s string) (int, error) {
	if len(s) != 1 || s[0] < '0' || s[0] > '9' {
		return 0, &ParseError{"bad " + s, len(s)}
	}
	return int(s[0] - '0'), nil
}

var errAgain = errors.New("again")

func attempt(attempts int) error {
	if attempts < 3 {
		return errAgain
	}
	return nil
}

func findDone(i int) int {
	for j := range 10 {
		if (i+j)%7 == 0 {
			return j
		}
	}
	return 0
}

func main() {
	ok, bad, ensured := 0, 0, 0
	inputs := []string{"1", "x", "3", "4", "y", "6", "7", "8", "z", "0"}
	for i := range 100_000 {
		n, err := parseDigit(inputs[i%10])
		var pe *ParseError
		if errors.As(err, &pe) {
			bad += pe.Pos
		} else {
			ok += n
		}
		ensured++ // ensure: nothing in between can panic, so no defer
	}
	fmt.Println(ok)
	fmt.Println(bad)
	fmt.Println(ensured)

	found := 0
	for i := range 50_000 {
		found += findDone(i)
	}
	fmt.Println(found)

	tries := 0
	for range 20_000 {
		attempts := 0
		for {
			attempts++
			if err := attempt(attempts); errors.Is(err, errAgain) {
				continue
			}
			tries += attempts
			break
		}
	}
	fmt.Println(tries)

	v := 0
	for i := range 100_000 {
		if n, err := strconv.Atoi(strconv.Itoa(i%50) + "x"); err != nil {
			v++
		} else {
			v += n
		}
		n, err := strconv.Atoi(strconv.Itoa(i % 50))
		if err != nil {
			log.Fatal(err)
		}
		v += n
	}
	fmt.Println(v)
}
