//go:build rugaroo_prelude

package prelude

import "slices"

// rbSortStable is slices.SortStableFunc at O(n log n): a run past one insertion-sort block is pdqsorted with ties
// broken by position, which is the same order. Up to 20 elements it is SortStableFunc itself, so a small sort compares
// the same pairs in the same order (a failure names the pair it does; decision 332).
func rbSortStable[E any](s []E, cmp func(a, b E) int) {
	if len(s) <= 20 {
		slices.SortStableFunc(s, cmp)
		return
	}
	type at struct {
		i int
		e E
	}
	t := make([]at, len(s))
	for i, e := range s {
		t[i] = at{i, e}
	}
	slices.SortFunc(t, func(a, b at) int {
		if c := cmp(a.e, b.e); c != 0 {
			return c
		}
		return a.i - b.i
	})
	for i := range t {
		s[i] = t[i].e
	}
}
