// Hand-written Go port of benchmarks/bignum.rb, for the README comparison column: math/big throughout.
package main

import (
	"fmt"
	"math/big"
	"strings"
)

func fibDigitSum(n int) int {
	a, b := big.NewInt(0), big.NewInt(1)
	for range n {
		a, b = b, new(big.Int).Add(a, b)
	}
	sum := 0
	for _, c := range a.String() {
		sum += int(c - '0')
	}
	return sum
}

func factorial(n int64) *big.Int {
	f := big.NewInt(1)
	for i := int64(2); i <= n; i++ {
		f.Mul(f, big.NewInt(i))
	}
	return f
}

func piDigits(count int) string {
	q, r, t := big.NewInt(1), big.NewInt(0), big.NewInt(1)
	k, n, l := int64(1), big.NewInt(3), int64(3)
	var out strings.Builder
	tmp, tmp2 := new(big.Int), new(big.Int)
	for out.Len() < count {
		// 4q + r - t < n t
		tmp.Mul(q, big.NewInt(4)).Add(tmp, r).Sub(tmp, t)
		tmp2.Mul(n, t)
		if tmp.Cmp(tmp2) < 0 {
			out.WriteString(n.String())
			nr := new(big.Int).Sub(r, tmp2)
			nr.Mul(nr, big.NewInt(10))
			nn := new(big.Int).Mul(q, big.NewInt(3))
			nn.Add(nn, r).Mul(nn, big.NewInt(10)).Div(nn, t)
			nn.Sub(nn, new(big.Int).Mul(n, big.NewInt(10)))
			n = nn
			q.Mul(q, big.NewInt(10))
			r = nr
		} else {
			nr := new(big.Int).Mul(q, big.NewInt(2))
			nr.Add(nr, r).Mul(nr, big.NewInt(l))
			nn := new(big.Int).Mul(q, big.NewInt(7*k))
			nn.Add(nn, big.NewInt(2)).Add(nn, new(big.Int).Mul(r, big.NewInt(l)))
			nn.Div(nn, new(big.Int).Mul(t, big.NewInt(l)))
			q.Mul(q, big.NewInt(k))
			t.Mul(t, big.NewInt(l))
			l += 2
			k++
			n = nn
			r = nr
		}
	}
	return out.String()
}

func main() {
	total := 0
	for i := range 200 {
		total += fibDigitSum(500 + i)
	}
	fmt.Println(total)
	fmt.Println(len(factorial(2000).String()))
	digits := piDigits(1200)
	fmt.Println(digits[:20])
	fmt.Println(digits[len(digits)-20:])
}
