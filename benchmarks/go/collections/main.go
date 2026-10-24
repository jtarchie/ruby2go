// Hand-written Go port of benchmarks/collections.rb, for the README comparison column.
package main

import (
	"cmp"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type City string

type Person struct {
	Name string
	Age  int
	City City
}

type cityInt struct {
	City City
	N    int
}

func inspectPairs(ps []cityInt) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = "[:" + string(p.City) + ", " + strconv.Itoa(p.N) + "]"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func main() {
	cities := []City{"paris", "tokyo", "lima", "oslo", "cairo"}
	people := make([]Person, 100_000)
	for i := range people {
		people[i] = Person{"p" + strconv.Itoa(i*7919%100_003), 18 + (i * 31 % 60), cities[i%5]}
	}

	byAgeName := slices.Clone(people)
	slices.SortFunc(byAgeName, func(a, b Person) int {
		return cmp.Or(cmp.Compare(a.Age, b.Age), strings.Compare(a.Name, b.Name))
	})
	names := make([]string, 3)
	for i, p := range byAgeName[:3] {
		names[i] = strconv.Quote(p.Name)
	}
	fmt.Println("[" + strings.Join(names, ", ") + "]")

	byAgeDesc := slices.Clone(people)
	slices.SortFunc(byAgeDesc, func(a, b Person) int { return cmp.Compare(b.Age, a.Age) })
	fmt.Println(byAgeDesc[0].Age)

	byCity := map[City][]Person{}
	for _, p := range people {
		byCity[p.City] = append(byCity[p.City], p)
	}
	sizes := make([]cityInt, 0, len(byCity))
	for c, ps := range byCity {
		sizes = append(sizes, cityInt{c, len(ps)})
	}
	slices.SortFunc(sizes, func(a, b cityInt) int {
		return cmp.Or(strings.Compare(string(a.City), string(b.City)), cmp.Compare(a.N, b.N))
	})
	fmt.Println(inspectPairs(sizes))

	ages := make([]int, len(people))
	for i, p := range people {
		ages[i] = p.Age
	}
	tally := map[int]int{}
	var order []int // insertion order, so max_by's first-wins tie break matches Ruby
	for _, a := range ages {
		if _, ok := tally[a]; !ok {
			order = append(order, a)
		}
		tally[a]++
	}
	bestAge := order[0]
	for _, a := range order[1:] {
		if tally[a] > tally[bestAge] {
			bestAge = a
		}
	}
	fmt.Printf("[%d, %d]\n", bestAge, tally[bestAge])

	var young, old []Person
	for _, p := range people {
		if p.Age < 40 {
			young = append(young, p)
		} else {
			old = append(old, p)
		}
	}
	fmt.Println(len(young))
	fmt.Println(len(old))

	minP, maxP := people[0], people[0]
	for _, p := range people[1:] {
		if p.Age < minP.Age {
			minP = p
		}
		if p.Name > maxP.Name {
			maxP = p
		}
	}
	fmt.Println(minP.Name)
	fmt.Println(maxP.Name)

	decades := make([]int, len(people))
	for i, p := range people {
		decades[i] = p.Age / 10
	}
	seenDecade := map[int]struct{}{}
	var uniq []int
	for _, d := range decades {
		if _, ok := seenDecade[d]; !ok {
			seenDecade[d] = struct{}{}
			uniq = append(uniq, d)
		}
	}
	slices.Sort(uniq)
	parts := make([]string, len(uniq))
	for i, d := range uniq {
		parts[i] = strconv.Itoa(d)
	}
	fmt.Println("[" + strings.Join(parts, ", ") + "]")

	seen := map[string]struct{}{}
	dups := 0
	for _, p := range people {
		prefix := p.Name[:min(4, len(p.Name))]
		if _, ok := seen[prefix]; ok {
			dups++
		} else {
			seen[prefix] = struct{}{}
		}
	}
	fmt.Println(len(seen))
	fmt.Println(dups)

	h := map[City]int{}
	for _, p := range people {
		h[p.City] += p.Age
	}
	sums := make([]cityInt, 0, len(h))
	for c, n := range h {
		sums = append(sums, cityInt{c, n})
	}
	slices.SortFunc(sums, func(a, b cityInt) int {
		return cmp.Or(cmp.Compare(b.N, a.N), strings.Compare(string(a.City), string(b.City)))
	})
	fmt.Println(inspectPairs(sums))

	flat := make([]int, 0, 2*len(people))
	for _, p := range people {
		flat = append(flat, p.Age, p.Age*2)
	}
	total := 0
	for _, x := range flat {
		total += x
	}
	fmt.Println(total)

	var sliceSums []int
	for i := 0; i < len(people); i += 1000 {
		s := 0
		for _, p := range people[i:min(i+1000, len(people))] {
			s += p.Age
		}
		sliceSums = append(sliceSums, s)
	}
	fmt.Println(slices.Max(sliceSums))

	var lima []Person
	for _, p := range people {
		if p.City == "lima" {
			lima = append(lima, p)
		}
	}
	var even []Person
	for _, p := range lima {
		if p.Age%2 == 0 {
			even = append(even, p)
		}
	}
	fmt.Println(len(even))

	xs := make([]int, 200_000)
	for i := range xs {
		xs[i] = i + 1
	}
	slices.Reverse(xs)
	slices.Sort(xs)
	fmt.Println(xs[sort.Search(len(xs), func(i int) bool { return xs[i] >= 123_456 })])
}
