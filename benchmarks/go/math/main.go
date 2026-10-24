// Hand-written Go port of benchmarks/math.rb, for the README comparison column.
package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// rubyFloat formats like Float#to_s for magnitudes in [1e-4, 1e16).
func rubyFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func main() {
	total := 0.0
	for i := range 200_000 {
		x := float64(i)
		total += math.Sin(x) * math.Cos(x)
	}
	fmt.Println(rubyFloat(math.Round(total*1e4) / 1e4))
}
