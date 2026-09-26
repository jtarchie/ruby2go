package main

import "fmt"

// ---- prelude (subset)
type String string
type Integer int

// Exception hierarchy: embedding promotes the marker methods, so
// `rescue AppError` matches *NotFound via an interface assertion.
type Exception struct{ message String }

func (self *Exception) Message() String { return self.message }
func (*Exception) isException()         {}

type StandardError struct{ Exception }

func (*StandardError) isStandardError() {}

type KeyError struct{ StandardError }

func (*KeyError) isKeyError() {}

type Hash[K comparable, V any] map[K]V

func (self Hash[K, V]) Fetch(k K) V {
	v, ok := self[k]
	if !ok {
		e := &KeyError{}
		e.message = String(fmt.Sprintf("key not found: %v", k))
		panic(e)
	}
	return v
}

// ---- user code
type AppError struct{ StandardError }

func (*AppError) isAppError() {}

type NotFound struct{ AppError }

func (*NotFound) isNotFound() {}

//line main.rb:6
func lookup(h Hash[String, Integer], k String) (ret Integer) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(interface{ isKeyError() }); ok { // rescue KeyError
					e := &NotFound{}
					e.message = "missing " + k
					panic(e)
				}
				panic(r) // not ours → keep unwinding
			}
		}()
		ret = h.Fetch(k)
	}()
	return
}

func main() {
//line main.rb:12
	func() {
		defer func() { fmt.Println("done") }() // ensure
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(interface {
					isAppError()
					Message() String
				}); ok { // rescue AppError => e
					fmt.Println("handled: " + e.Message())
					return
				}
				panic(r)
			}
		}()
		lookup(Hash[String, Integer]{"a": 1}, "b")
	}()
}
