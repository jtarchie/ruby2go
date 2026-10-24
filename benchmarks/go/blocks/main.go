// Hand-written Go port of benchmarks/blocks.rb, for the README comparison column.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

func foldUp(n int, f func(int) int) int {
	acc := 0
	for i := 0; i < n; i++ {
		acc += f(i)
	}
	return acc
}

func adder(k int) func(int) int { return func(x int) int { return x + k } }

func compose(f, g func(int) int) func(int) int { return func(x int) int { return g(f(x)) } }

func main() {
	total := 0
	for j := range 20 {
		total += foldUp(20_000, func(i int) int { return (i ^ j) & 255 })
	}
	fmt.Println(total)

	add3 := adder(3)
	twice := func(x int) int { return x * 2 }
	composed := compose(add3, twice)
	sum := 0
	for i := 1; i <= 300_000; i++ {
		sum += composed(i) % 11
	}
	fmt.Println(sum)

	counter := 0
	bump := func() { counter++ }
	for range 200_000 {
		bump()
	}
	fmt.Println(counter)

	nums := make([]int, 300_000)
	for i := range nums {
		nums[i] = i + 1
	}

	acc := 0
	for _, n := range nums {
		acc += n % 7
	}
	fmt.Println(acc)

	h := map[int]int{}
	for _, n := range nums {
		h[n%13]++
	}
	maxV := 0
	first := true
	for _, v := range h {
		if first || v > maxV {
			maxV, first = v, false
		}
	}
	fmt.Println(maxV)

	sum = 0
	for i := 0; i < len(nums); i += 4 {
		s := nums[i:min(i+4, len(nums))]
		sum += s[0]
	}
	fmt.Println(sum)

	count := 0
	for i := 0; i+3 <= len(nums); i++ {
		w := nums[i : i+3]
		if (w[0]+w[1]+w[2])%5 == 0 {
			count++
		}
	}
	fmt.Println(count)

	sum = 0
	for i, n := range nums {
		sum += n * (i % 3)
	}
	fmt.Println(sum % 1_000_003)

	var firstFive []int
	for _, n := range nums {
		m := n * 3
		if m%2 == 0 {
			firstFive = append(firstFive, m)
			if len(firstFive) == 5 {
				break
			}
		}
	}
	fmt.Println(inspectInts(firstFive))

	rev := make([]int, len(nums))
	for i, n := range nums {
		rev[len(nums)-1-i] = n
	}
	zipped := make([][2]int, len(nums))
	for i := range nums {
		zipped[i] = [2]int{nums[i], rev[i]}
	}
	sum = 0
	for _, p := range zipped {
		sum += p[0] * p[1] % 17
	}
	fmt.Println(sum)
}

func inspectInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
