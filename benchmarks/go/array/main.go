// Hand-written Go port of benchmarks/array.rb, for the README comparison column.
package main

import "fmt"

func main() {
	squares := make([]int, 0, 1_000_000)
	for i := 1; i <= 1_000_000; i++ {
		squares = append(squares, i*i)
	}
	var evens []int
	for _, v := range squares {
		if v%2 == 0 {
			evens = append(evens, v)
		}
	}
	sum := 0
	for _, v := range evens {
		sum += v
	}
	fmt.Println(sum)
	fmt.Println(len(evens))
}
