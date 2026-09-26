package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ---- prelude (subset)
type String string
type Float float64

func (self String) Plus(o String) String { return self + o }
func (self Float) Mul(o Float) Float     { return self * o }
func (self Float) ToS() String {
	s := strconv.FormatFloat(float64(self), 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0" // Go prints 6, Ruby prints 6.0
	}
	return String(s)
}

// ---- user code

// Every class gets an interface of its full (inherited + own) method set.
// Methods defined on Shape take it, so `area`/`name` dispatch to the subclass.
type ShapeI interface {
	Name() String
	Area() Float
	Describe() String
}

type Shape struct{}

//line main.rb:4
func Shape_Name(self ShapeI) String { return "Shape" }

//line main.rb:7
func Shape_Area(self ShapeI) Float { panic("NotImplementedError") }

//line main.rb:10
func Shape_Describe(self ShapeI) String { return self.Name().Plus(": ").Plus(self.Area().ToS()) }

type Rect struct {
	Shape
	w, h Float
}

//line main.rb:18
func Rect_Initialize(self *Rect, w, h Float) { self.w = w; self.h = h }
func NewRect(w, h Float) *Rect               { self := &Rect{}; Rect_Initialize(self, w, h); return self }

func (self *Rect) W() Float { return self.w }
func (self *Rect) H() Float { return self.h }

//line main.rb:23
func (self *Rect) Name() String { return "Rect" }

//line main.rb:24
func (self *Rect) Area() Float { return self.W().Mul(self.H()) }

// Forwarder re-emitted per class. Relying on embedding would bind
// self to the embedded *Rect and Square#describe would print "Rect".
func (self *Rect) Describe() String { return Shape_Describe(self) }

type Square struct{ Rect }

//line main.rb:29
func Square_Initialize(self *Square, side Float) { Rect_Initialize(&self.Rect, side, side) } // super
func NewSquare(side Float) *Square               { self := &Square{}; Square_Initialize(self, side); return self }

//line main.rb:31
func (self *Square) Name() String     { return "Square" }
func (self *Square) Describe() String { return Shape_Describe(self) }

func main() {
//line main.rb:34
	shapes := []ShapeI{NewRect(2.0, 3.0), NewSquare(2.0)}
	for _, s := range shapes {
		fmt.Println(s.Describe())
	}
}
