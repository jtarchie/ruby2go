package rugaroo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// powCheck goes beside a generated main.go: it compares rbFloatPow, fast paths and all, with Float#**'s former body (the special cases, then rbFloatPowExact) on random pairs of every kind (decision 390).
const powCheck = `package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
)

func powOld(x, y float64) float64 {
	switch {
	case y == 2:
		return x * x
	case x == 0 || y == 0 || x == 1 || x == -1 || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0),
		x < 0 && y != math.Trunc(y):
		return math.Pow(x, y)
	}
	return rbFloatPowExact(x, y)
}

func powX(r *rand.Rand, lo, hi int) float64 { return math.Ldexp(1+r.Float64(), lo+r.IntN(hi-lo+1)) }

func powSign(r *rand.Rand, x float64) float64 {
	if r.IntN(2) == 0 {
		return -x
	}
	return x
}

func powForP(x, p float64) float64 { return p / math.Log(math.Abs(x)) }

var powKinds = []func(r *rand.Rand) (float64, float64){
	func(r *rand.Rand) (float64, float64) { return math.Float64frombits(r.Uint64()), math.Float64frombits(r.Uint64()) },
	func(r *rand.Rand) (float64, float64) { return math.Float64frombits(r.Uint64()), powSign(r, powX(r, -30, 3)) },
	func(r *rand.Rand) (float64, float64) { return powX(r, -10, 9), (r.Float64()*2 - 1) * 100 },
	func(r *rand.Rand) (float64, float64) { return powSign(r, powX(r, -20, 19)), float64(r.IntN(129) - 64) },
	func(r *rand.Rand) (float64, float64) { return -powX(r, -6, 5), float64(r.IntN(401) - 200) },
	func(r *rand.Rand) (float64, float64) {
		x := 1 + powSign(r, math.Ldexp(1+r.Float64(), -1-r.IntN(52)))
		return x, powForP(x, (r.Float64()*2-1)*700)
	},
	func(r *rand.Rand) (float64, float64) {
		x := math.Float64frombits(r.Uint64()>>12 | 1)
		return x, powForP(x, (r.Float64()*2-1)*700)
	},
	func(r *rand.Rand) (float64, float64) {
		edges := []float64{-708.39, 709.79, -708, 709, -760, 760}
		x := powX(r, -40, 39) + 0.5
		return x, powForP(x, edges[r.IntN(len(edges))]+(r.Float64()*2-1)*0.02)
	},
}

func init() {
	if os.Getenv("RUGAROO_POWCHECK") == "" {
		return
	}
	r := rand.New(rand.NewPCG(1, 2))
	bad, inRange, fast := 0, 0, 0
	for _, kind := range powKinds {
		for range 150000 {
			x, y := kind(r)
			got, want := rbFloatPow(x, y), powOld(x, y)
			if math.Float64bits(got) != math.Float64bits(want) && !(math.IsNaN(got) && math.IsNaN(want)) {
				bad++
				if bad <= 10 {
					fmt.Printf("%x ** %x = %x, want %x\n", x, y, got, want)
				}
			}
			if p := y * math.Log(math.Abs(x)); x > 0 && x != 1 && y != 0 && p > -700 && p < 700 {
				inRange++
				if _, ok := rbPowFast(x, y); ok {
					fast++
				}
			}
		}
	}
	fmt.Printf("mismatches %d\n", bad)
	if fast < inRange*99/100 {
		fmt.Printf("the fast path answered %d of %d in-range calls\n", fast, inRange)
	}
	os.Exit(0)
}
`

func TestFloatPowFastPaths(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	gen := transpile(t, "float_pow.rb", []byte("puts ARGV.size.to_f ** 0.5\n"))
	err := os.WriteFile(filepath.Join(gen, "powcheck.go"), []byte(powCheck), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	bin := goBuild(t, gen, "")
	got := binOutput(t, gen, progIO{env: []string{"RUGAROO_POWCHECK=1"}}, bin, nil)
	if strings.TrimSpace(got.Stdout) != "mismatches 0" || got.Code != 0 {
		t.Errorf("rbFloatPow differs from rbFloatPowExact (exit %d):\n%s%s", got.Code, got.Stdout, got.Stderr)
	}
}
