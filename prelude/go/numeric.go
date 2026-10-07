//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

// rbNumTowerLevel is v's place in MRI's coercion order Integer < Rational < Float < BigDecimal < Complex (decision 142), -1 for a non-number.
func rbNumTowerLevel(v any) int {
	switch v.(type) { // one type per case, so a class the program never makes costs nothing (the pruner drops its case)
	case Integer:
		return 0
	case *Rational:
		return 1
	case Float:
		return 2
	case *BigDecimal:
		return 3
	case *Complex:
		return 4
	default: // gocritic rejects a one-case switch, which pruning can leave
		return -1
	}
}

// rbNumOther reports an argument that is a number of another class than self, which Comparable's methods then compare through rbNum.
func rbNumOther(self any, args []any) bool {
	l := rbNumTowerLevel(self)
	for _, a := range args {
		if k := rbNumTowerLevel(rbUnbox(a)); k >= 0 && k != l {
			return true
		}
	}
	return false
}

// rbNumCoerce converts two numbers of different classes to the higher one's, as MRI's coerce; false when they are not both numbers or a BigDecimal meets a Complex.
func rbNumCoerce(a, b any) (any, any, bool) {
	b = rbUnbox(b)
	la, lb := rbNumTowerLevel(a), rbNumTowerLevel(b)
	if la < 0 || lb < 0 || la == lb {
		return nil, nil, false
	}
	hi := a
	if lb > la {
		hi = b
	}
	switch h := hi.(type) {
	case *Rational:
		return rbNumTo(a, 1), rbNumTo(b, 1), true
	case Float:
		return Float(rbNumFloat(a)), Float(rbNumFloat(b)), true
	case *BigDecimal:
		prec := rbBDCoercePrec(h) // as rbBDRel: a fixed precision loses digits of a longer BigDecimal
		return rbBDArg(a, prec), rbBDArg(b, prec), true
	case *Complex:
		return rbNumComplex(a), rbNumComplex(b), la != 3 && lb != 3
	default: // gocritic rejects a one-case switch, which pruning can leave
		return nil, nil, false
	}
}

// rbNumComplex is a real number as a Complex, or a Complex itself.
func rbNumComplex(v any) any {
	switch v.(type) {
	case Integer, Float, *Rational:
		return &Complex{v, Integer(0)}
	}
	return v
}
