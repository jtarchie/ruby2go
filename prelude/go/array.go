//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbFrom converts v, an Array of any instantiation, into this one: a
// copy, each element converted (rbConv). The receiver only names the
// instantiation.
func (*Array[E]) rbFrom(v any) (*Array[E], bool) {
	a, ok := v.(Array_Any)
	if !ok {
		return nil, false
	}
	src := a._ToAny()
	if out, ok := any(src).(*Array[E]); ok {
		return out, true // E is untyped: _ToAny copied
	}
	out := make(Array[E], 0, len(*src))
	for _, x := range *src {
		e, ok := rbConv[E](x)
		if !ok {
			return nil, false
		}
		out = append(out, e)
	}
	return &out, true
}

func NewArray[E comparable]() *Array[E] { return &Array[E]{} }

// rbSplatAt is element i of a yielded Array splatted across block params, or nil past its end.
func rbSplatAt[E comparable](a *Array[E], i int) *E {
	if a == nil || i >= len(*a) {
		return nil
	}
	return &(*a)[i]
}

// rbCombinations is every k-combination of all, in MRI's order.
func rbCombinations[E comparable](all []E, k int) *Array[*Array[E]] {
	out := &Array[*Array[E]]{}
	n := len(all)
	if k < 0 || k > n {
		return out
	}
	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	for {
		c := make(Array[E], k)
		for i, j := range idx {
			c[i] = all[j]
		}
		*out = append(*out, &c)
		i := k - 1
		for i >= 0 && idx[i] == n-k+i {
			i--
		}
		if i < 0 {
			return out
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
}

// rbPermutations is every k-permutation of all, in MRI's order.
func rbPermutations[E comparable](all []E, k int) *Array[*Array[E]] {
	out := &Array[*Array[E]]{}
	n := len(all)
	if k < 0 || k > n {
		return out
	}
	used := make([]bool, n)
	cur := make([]E, 0, k)
	var rec func()
	rec = func() {
		if len(cur) == k {
			c := Array[E](slices.Clone(cur))
			*out = append(*out, &c)
			return
		}
		for i := range n {
			if used[i] {
				continue
			}
			used[i] = true
			cur = append(cur, all[i])
			rec()
			cur = cur[:len(cur)-1]
			used[i] = false
		}
	}
	rec()
	return out
}

// rbFlattenInto appends xs to out, splicing in Arrays (any instantiation,
// tuples too) down to depth levels; depth < 0 is all of them.
// ponytail: a self-containing array overflows here, MRI raises ArgumentError; add a visited set.
func rbFlattenInto(out *Array[any], xs []any, depth int) {
	for _, x := range xs {
		if a, ok := x.(Array_Any); ok && depth != 0 {
			rbFlattenInto(out, *a._ToAny(), depth-1)
			continue
		}
		*out = append(*out, x)
	}
}

// rbFlattenRows is one level of flatten over typed rows. When U is untyped
// the rest of depth flattens dynamically; a typed U that is itself an
// Array only arises past what flatten's @self forms cover.
func rbFlattenRows[U comparable](rows []*Array[U], depth int) *Array[U] {
	out := &Array[U]{}
	for _, r := range rows {
		*out = append(*out, *r...)
	}
	if depth == 1 {
		return out
	}
	if a, ok := any(out).(*Array[any]); ok {
		flat := &Array[any]{}
		rbFlattenInto(flat, *a, depth-1)
		return any(flat).(*Array[U])
	}
	var z U
	if _, ok := any(z).(Array_Any); ok {
		panic(NewNotImplementedError(Ref(String("rb2go: flatten of this depth over typed Arrays nested this deep"))))
	}
	return out
}

// rbRepeated is Array#repeated_combination (combo) or #repeated_permutation, eagerly in MRI's order.
func rbRepeated[E comparable](all []E, k int, combo bool) *Array[*Array[E]] {
	out := &Array[*Array[E]]{}
	n := len(all)
	if k < 0 || n == 0 && k > 0 {
		return out
	}
	idx := make([]int, k)
	for {
		c := make(Array[E], k)
		for i, j := range idx {
			c[i] = all[j]
		}
		*out = append(*out, &c)
		i := k - 1
		for i >= 0 && idx[i] == n-1 {
			i--
		}
		if i < 0 {
			return out
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			if combo {
				idx[j] = idx[i]
			} else {
				idx[j] = 0
			}
		}
	}
}

// rbMidSplat is `a, *mid, z = arr`'s mid: what the lead and trail targets leave, possibly empty.
func rbMidSplat[E comparable](a *Array[E], lead, trail int) *Array[E] {
	end := max(len(*a)-trail, lead)
	if lead >= len(*a) {
		return &Array[E]{}
	}
	out := append(Array[E]{}, (*a)[lead:end]...)
	return &out
}

// rbTrailIdx is trailing target j's element: counted from the end, never overlapping the leading targets.
func rbTrailIdx[E comparable](a *Array[E], lead, trail, j int) *E {
	i := max(len(*a)-trail, lead) + j
	if i >= len(*a) {
		return nil
	}
	return &(*a)[i]
}

// rbAssoc is Array#assoc (at 0) and #rassoc (at 1): elements that are not Arrays are skipped.
func rbAssoc[E comparable](a *Array[E], key any, at int) *E {
	key = rbUnbox(key)
	for i := range *a {
		if x, ok := any((*a)[i]).(interface{ _ToAny() *Array[any] }); ok {
			if xs := *x._ToAny(); len(xs) > at && bool(rbEq[any](xs[at], key)) {
				return &(*a)[i]
			}
		}
	}
	return nil
}

// rbPack is Array#pack for C, c and U, each with a count or *, MRI's errors included.
func rbPack(items []any, format string) string {
	var out []byte
	next := 0
	for i := 0; i < len(format); {
		d := format[i]
		i++
		if d == ' ' || d == '\t' || d == '\n' {
			continue
		}
		count := 1
		switch j := i; {
		case i < len(format) && format[i] == '*':
			count = len(items) - next
			i++
		default:
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			if j > i {
				count, _ = strconv.Atoi(format[i:j])
				i = j
			}
		}
		if d != 'C' && d != 'c' && d != 'U' {
			panic(NewNotImplementedError(Ref(String("pack directive '" + string(d) + "' is not supported by rb2go"))))
		}
		for range count {
			if next >= len(items) {
				panic(NewArgumentError(Ref(String("too few arguments"))))
			}
			n, ok := rbUnbox(items[next]).(Integer)
			if !ok {
				panic(rbConvError(items[next], "Integer"))
			}
			next++
			switch {
			case d != 'U':
				out = append(out, byte(n))
			case n < 0:
				panic(NewRangeError(Ref(String("pack(U): value out of range"))))
			default:
				out = utf8.AppendRune(out, rune(n))
			}
		}
	}
	return string(out)
}
