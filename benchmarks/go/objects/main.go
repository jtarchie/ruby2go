// Hand-written Go port of benchmarks/objects.rb, for the README comparison column.
package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func rubyFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func round(f float64, digits int) float64 {
	p := math.Pow10(digits)
	return math.Round(f*p) / p
}

type Vec struct{ X, Y float64 }

func (v Vec) Add(o Vec) Vec       { return Vec{v.X + o.X, v.Y + o.Y} }
func (v Vec) Sub(o Vec) Vec       { return Vec{v.X - o.X, v.Y - o.Y} }
func (v Vec) Scale(k float64) Vec { return Vec{v.X * k, v.Y * k} }
func (v Vec) Length() float64     { return math.Sqrt(v.X*v.X + v.Y*v.Y) }

type Body struct {
	Pos, Vel Vec
	Mass     float64
}

type Shape interface{ Area() float64 }

type Circle struct{ R float64 }

func (c Circle) Area() float64 { return 3.14159 * c.R * c.R }

type Rect struct{ W, H float64 }

func (r Rect) Area() float64 { return r.W * r.H }

type Square struct{ Rect }

func NewSquare(s float64) Square { return Square{Rect{s, s}} }

func main() {
	bodies := make([]*Body, 40)
	for i := range bodies {
		bodies[i] = &Body{Pos: Vec{float64(i), float64(i * 3 % 7)}, Mass: 1.0 + float64(i%3)}
	}
	for range 60 {
		for _, a := range bodies {
			var acc Vec
			for _, b := range bodies {
				if a == b {
					continue
				}
				d := b.Pos.Sub(a.Pos)
				dist := d.Length() + 0.01
				acc = acc.Add(d.Scale(b.Mass / (dist * dist * dist)))
			}
			a.Vel = a.Vel.Add(acc.Scale(0.001))
		}
		for _, b := range bodies {
			b.Pos = b.Pos.Add(b.Vel)
		}
	}
	sum := 0.0
	for _, b := range bodies {
		sum += b.Pos.Length()
	}
	fmt.Println(rubyFloat(round(sum, 6)))

	shapes := make([]Shape, 0, 200_000)
	for i := range 200_000 {
		var s Shape
		switch i % 3 {
		case 0:
			s = Circle{float64(i % 10)}
		case 1:
			s = Rect{float64(i % 7), float64(i % 5)}
		default:
			s = NewSquare(float64(i % 4))
		}
		shapes = append(shapes, s)
	}
	total := 0.0
	for _, s := range shapes {
		total += s.Area()
	}
	fmt.Println(rubyFloat(round(total, 2)))
	maxS := shapes[0]
	for _, s := range shapes[1:] {
		if s.Area() > maxS.Area() {
			maxS = s
		}
	}
	fmt.Println(rubyFloat(maxS.Area()))
	ref := shapes[1]
	count := 0
	for _, s := range shapes {
		if s.Area() > ref.Area() {
			count++
		}
	}
	fmt.Println(count)
}
