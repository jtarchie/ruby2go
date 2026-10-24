// Hand-written Go port of benchmarks/hash.rb, for the README comparison column.
package main

import "fmt"

func main() {
	counts := map[int]int{}
	for i := range 500_000 {
		counts[i%1000]++
	}
	fmt.Println(len(counts))
	values := make([]int, 0, len(counts))
	for _, v := range counts {
		values = append(values, v)
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	fmt.Println(sum)
}
