//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbMT is MRI's MT19937, seeded the way MRI's rand_init does, so seeded sequences match.
type rbMT struct {
	mu sync.Mutex
	s  [624]uint32
	i  int
}

func (m *rbMT) initGen(seed uint32) {
	m.s[0] = seed
	for i := 1; i < 624; i++ {
		m.s[i] = 1812433253*(m.s[i-1]^(m.s[i-1]>>30)) + uint32(i)
	}
	m.i = 624
}

func (m *rbMT) initArray(key []uint32) {
	m.initGen(19650218)
	i, j := 1, 0
	for k := max(624, len(key)); k > 0; k-- {
		m.s[i] = (m.s[i] ^ ((m.s[i-1] ^ (m.s[i-1] >> 30)) * 1664525)) + key[j] + uint32(j)
		i++
		j++
		if i >= 624 {
			m.s[0] = m.s[623]
			i = 1
		}
		if j >= len(key) {
			j = 0
		}
	}
	for k := 623; k > 0; k-- {
		m.s[i] = (m.s[i] ^ ((m.s[i-1] ^ (m.s[i-1] >> 30)) * 1566083941)) - uint32(i)
		i++
		if i >= 624 {
			m.s[0] = m.s[623]
			i = 1
		}
	}
	m.s[0] = 0x80000000
}

// seed packs |seed| into 32-bit words, low first: one word seeds init_genrand, more init_by_array.
func (m *rbMT) seed(seed int) {
	u := uint64(seed)
	if seed < 0 {
		u = uint64(-seed)
	}
	if u>>32 == 0 {
		m.initGen(uint32(u))
		return
	}
	m.initArray([]uint32{uint32(u), uint32(u >> 32)})
}

func (m *rbMT) next() uint32 {
	if m.i >= 624 {
		for k := range 624 {
			y := (m.s[k] & 0x80000000) | (m.s[(k+1)%624] & 0x7fffffff)
			v := m.s[(k+397)%624] ^ (y >> 1)
			if y&1 != 0 {
				v ^= 0x9908b0df
			}
			m.s[k] = v
		}
		m.i = 0
	}
	y := m.s[m.i]
	m.i++
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	return y
}

// real is genrand_res53: a Float in [0, 1).
func (m *rbMT) real() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, b := m.next()>>5, m.next()>>6
	return (float64(a)*67108864 + float64(b)) * (1.0 / 9007199254740992.0)
}

// limited is MRI's limited_rand: uniform in [0, limit], by masked rejection.
func (m *rbMT) limited(limit uint64) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit == 0 {
		return 0
	}
	mask := limit
	for s := 1; s < 64; s <<= 1 {
		mask |= mask >> s
	}
retry:
	var val uint64
	for i := 1; i >= 0; i-- {
		if (mask>>(uint(i)*32))&0xffffffff != 0 {
			val |= uint64(m.next()) << (uint(i) * 32)
			val &= mask
			if limit < val {
				goto retry
			}
		}
	}
	return val
}

func (m *rbMT) upto(n int) int { return int(m.limited(uint64(n - 1))) }

func rbNewSeed() Integer {
	var b [8]byte
	_, _ = rand.Read(b[:]) // never fails (crypto/rand docs)
	return Integer(binary.LittleEndian.Uint64(b[:]) >> 2)
}

func rbNewRandom(seed Integer) *Random {
	r := &Random{seed: seed}
	r.mt.seed(int(seed))
	return r
}

var rbDefaultRandom = rbNewRandom(rbNewSeed())

func rbShuffle[E any](xs []E, m *rbMT) {
	for i := len(xs); i > 0; {
		j := m.upto(i)
		i--
		xs[i], xs[j] = xs[j], xs[i]
	}
}

// rbSampleIdx is MRI's Array#sample index choice: special cases up to 3, sorted insertion up to 10, else a partial shuffle.
func rbSampleIdx(n, size int, m *rbMT) []int {
	n = min(n, size)
	switch {
	case n <= 0:
		return nil
	case n == 1:
		return []int{m.upto(size)}
	case n == 2:
		i, j := m.upto(size), m.upto(size-1)
		if j >= i {
			j++
		}
		return []int{i, j}
	case n == 3:
		i, j, k := m.upto(size), m.upto(size-1), m.upto(size-2)
		l, g := j, i
		if j >= i {
			l = i
			j++
			g = j
		}
		if k >= l {
			k++
			if k >= g {
				k++
			}
		}
		return []int{i, j, k}
	case n <= 10:
		idx := []int{m.upto(size)}
		sorted := []int{idx[0]}
		for i, left := 1, size; i < n; i++ {
			left--
			k := m.upto(left)
			j := 0
			for ; j < i; j++ {
				if k < sorted[j] {
					break
				}
				k++
			}
			sorted = slices.Insert(sorted, j, k)
			idx = append(idx, k)
		}
		return idx
	}
	perm := make([]int, size)
	for i := range perm {
		perm[i] = i
	}
	for i := range n {
		j := m.upto(size-i) + i
		perm[i], perm[j] = perm[j], perm[i]
	}
	return perm[:n]
}

func rbRandArg(n Integer) {
	if n <= 0 {
		panic(NewArgumentError(Ref(String("invalid argument - " + strconv.Itoa(int(n))))))
	}
}
