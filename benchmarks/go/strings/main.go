// Hand-written Go port of benchmarks/strings.rb, for the README comparison column.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

func main() {
	parts := make([]string, 0, 100_000)
	for i := range 100_000 {
		parts = append(parts, "item-"+strconv.Itoa(i)+"-"+strconv.Itoa(i*2))
	}
	joined := strings.Join(parts, ",")
	words := strings.Split(joined, ",")
	total := 0
	for _, w := range words {
		total += len(w)
	}
	fmt.Println(total)
	fmt.Println(words[0])
}
