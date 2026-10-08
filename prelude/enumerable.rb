# prelude/enumerable.rb
# rbs_inline: enabled
#
# Enumerable, written once against `each`.

# @rbs generic E
module Enumerable
  #: () { (E) -> void } -> void
  def each = raise(NotImplementedError)

  #: [U] () { (E) -> U } -> Array[U]
  def map
    out = [] #: Array[U]
    each { |x| out << yield(x) }
    out
  end

  # The elements where pattern === x: a Regexp, a class, a Range, a value.
  # @dynamic
  #: (untyped) -> Array[E]
  def grep(pattern)
    out = [] #: Array[E]
    each { |x| out << x if pattern === x }
    out
  end

  # @dynamic
  #: (untyped) -> Array[E]
  def grep_v(pattern)
    out = [] #: Array[E]
    each { |x| out << x unless pattern === x }
    out
  end

  #: () { (E) -> bool } -> Array[E]
  def select
    out = [] #: Array[E]
    each { |x| out << x if yield(x) }
    out
  end

  #: () { (E) -> bool } -> Array[E]
  def reject
    out = [] #: Array[E]
    each { |x| out << x unless yield(x) }
    out
  end

  #: () { (E) -> bool } -> E?
  def find
    each { |x| return x if yield(x) }
    nil
  end

  #: () { (E) -> bool } -> E?
  def detect
    each { |x| return x if yield(x) }
    nil
  end

  #: () { (E) -> bool } -> bool
  def any?
    each { |x| return true if yield(x) }
    false
  end

  #: () { (E) -> bool } -> bool
  def all?
    each { |x| return false unless yield(x) }
    true
  end

  #: () { (E) -> bool } -> bool
  def none?
    each { |x| return false if yield(x) }
    true
  end

  #: () { (E) -> bool } -> Integer
  def count_if
    n = 0
    each { |x| n += 1 if yield(x) }
    n
  end

  #: () -> Integer
  def count
    n = 0
    each { |_x| n += 1 }
    n
  end

  #: [A] (A) { (A, E) -> A } -> A
  def reduce(init)
    acc = init
    each { |x| acc = yield(acc, x) }
    acc
  end

  #: [A] (A) { (A, E) -> A } -> A
  def inject(init)
    acc = init
    each { |x| acc = yield(acc, x) }
    acc
  end

  #: () { (E) -> void } -> void
  def each_entry = each { |x| yield x }

  #: () { (E, Integer) -> void } -> void
  def each_with_index = %x{
    return func(yield func(E, Integer) bool) {
      i := Integer(0)
      for x := range self.Each() {
        if !yield(x, i) {
          return
        }
        i++
      }
    }
  }

  #: (untyped) -> bool
  def include?(v)
    each { |x| return true if x == v }
    false
  end

  #: () -> Array[E]
  def uniq = to_a.uniq

  #: () -> Array[E]
  def entries = to_a

  #: () -> Enumerator::Lazy[E]
  def lazy = %x{ return rbLazy(self.Each(), any(self), "") }

  #: () -> Array[E]
  def to_a
    out = [] #: Array[E]
    each { |x| out << x }
    out
  end

  # A free func, never a forwarder on Array[E], so Go sees no Array → Set → Hash → Array instantiation cycle (decision 139).
  #: () -> Set[E]
  def to_set = Set.new(to_a)

  #: [U] () { (E) -> U } -> Set[U]
  def __to_set_block
    out = [] #: Array[U]
    each { |x| out << yield(x) }
    Set.new(out)
  end

  #: () -> Hash[E, Integer]
  def tally
    out = {} #: Hash[E, Integer]
    each { |x| out[x] = (out[x] || 0) + 1 }
    out
  end

  #: (Integer) -> Array[E]
  def first(n)
    raise ArgumentError, __negative_first if n < 0
    out = [] #: Array[E]
    return out if n == 0
    each do |x|
      out << x
      break if out.size >= n # before the next element: a generator stops where MRI's does
    end
    out
  end

  # first's message varies by class (Array overrides it), take's does not.
  # MRI's sum: Integers and Rationals exactly, Floats with Kahan-Babuska compensation.
  # An Integer or Float element type sums unboxed (any(x).(Integer) does not
  # allocate once instantiated); anything else goes through rbSummer's tower.
  #: () -> E
  def sum = %x{
    var z E
    switch any(z).(type) {
    case Integer:
      var t Integer
      for x := range self.Each() {
        t = t.Op_plus(any(x).(Integer))
      }
      return any(t).(E)
    case Float:
      s := rbSummer{floating: true}
      for x := range self.Each() {
        s.addFloat(float64(any(x).(Float)))
      }
      return any(Float(s.f + s.c)).(E)
    }
    s := rbSummer{}
    for x := range self.Each() {
      s.add(any(x))
    }
    return rbAs[E](s.result(), "numeric")
  }

  #: [N] () { (E) -> N } -> N
  def __sum_block = %x{
    var z N
    switch any(z).(type) {
    case Integer:
      var t Integer
      for x := range self.Each() {
        t = t.Op_plus(any(blk(x)).(Integer))
      }
      return any(t).(N)
    case Float:
      s := rbSummer{floating: true}
      for x := range self.Each() {
        s.addFloat(float64(any(blk(x)).(Float)))
      }
      return any(Float(s.f + s.c)).(N)
    }
    s := rbSummer{}
    for x := range self.Each() {
      s.add(any(blk(x)))
    }
    return rbAs[N](s.result(), "numeric")
  }

  #: () -> E?
  def __first_0
    each { |x| return x }
    nil
  end

  #: (Integer) -> Array[E]
  def take(n)
    raise ArgumentError, "attempt to take negative size" if n < 0
    first(n)
  end

  #: () -> String
  def __negative_first = "attempt to take negative size"

  # Keys are computed once, then sorted by <=>. Stable, unlike MRI.
  #: [K] () { (E) -> K } -> Array[E]
  def sort_by = %x{
    type kv struct {
      k K
      v E
    }
    tmp := []kv{}
    for x := range self.Each() {
      tmp = append(tmp, kv{blk(x), x})
    }
    slices.SortStableFunc(tmp, func(a, b kv) int { return -int(rbCmp(b.k, a.k)) })
    out := &Array[E]{}
    for _, p := range tmp {
      out.s = append(out.s, p.v)
    }
    return out
  }

  #: () -> Array[E]
  def sort = %x{
    out := &Array[E]{}
    for x := range self.Each() {
      out.s = append(out.s, x)
    }
    slices.SortStableFunc(out.s, func(a, b E) int { return -int(rbCmp(b, a)) }) // (earlier, later), as MRI's failure names them
    return out
  }

  #: () -> E?
  def min = %x{
    var best *E
    for x := range self.Each() {
      if best == nil || rbCmp(*best, x) > 0 { // (best, candidate), as MRI's failure names them
        x := x
        best = &x
      }
    }
    return best
  }

  #: () -> E?
  def max = %x{
    var best *E
    for x := range self.Each() {
      if best == nil || rbCmp(*best, x) < 0 {
        x := x
        best = &x
      }
    }
    return best
  }

  #: [K] () { (E) -> K } -> Hash[K, Array[E]]
  def group_by
    out = {} #: Hash[K, Array[E]]
    each do |x|
      k = yield(x)
      bucket = out[k]
      if bucket
        bucket << x
      else
        out[k] = [x]
      end
    end
    out
  end

  #: [K] () { (E) -> K } -> E?
  def min_by = %x{
    var best *E
    var bestK K
    for x := range self.Each() {
      k := blk(x)
      if best == nil || rbCmp(bestK, k) > 0 {
        x := x
        best, bestK = &x, k
      }
    }
    return best
  }

  #: [K] () { (E) -> K } -> E?
  def max_by = %x{
    var best *E
    var bestK K
    for x := range self.Each() {
      k := blk(x)
      if best == nil || rbCmp(bestK, k) < 0 {
        x := x
        best, bestK = &x, k
      }
    }
    return best
  }

  #: [U] () { (E) -> Array[U] } -> Array[U]
  def flat_map
    out = [] #: Array[U]
    each { |x| yield(x).each { |y| out << y } }
    out
  end

  #: () -> Array[[E, Integer]]
  def with_index_pairs
    out = [] #: Array[[E, Integer]]
    i = 0
    each do |x|
      out << [x, i]
      i += 1
    end
    out
  end
  #: () { (E) -> bool } -> Integer
  def __count_block
    n = 0
    each { |x| n += 1 if yield(x) }
    n
  end

  #: (E) -> Integer
  def __count_1(v)
    n = 0
    each { |x| n += 1 if x == v }
    n
  end

  # No initial value: the first element starts the fold; nil when empty.
  #: () { (E, E) -> E } -> E?
  def __inject_0 = %x{
    var acc *E
    for x := range self.Each() {
      if acc == nil {
        x := x
        acc = &x
        continue
      }
      v := blk(*acc, x)
      acc = &v
    }
    return acc
  }

  #: () { (E, E) -> E } -> E?
  def __reduce_0 = %x{
    var acc *E
    for x := range self.Each() {
      if acc == nil {
        x := x
        acc = &x
        continue
      }
      v := blk(*acc, x)
      acc = &v
    }
    return acc
  }

  #: [A] (A) { (E, A) -> void } -> A
  def each_with_object(memo)
    each { |x| yield(x, memo) }
    memo
  end

  # Keeps the block's truthy results.
  #: [U] () { (E) -> U? } -> Array[U]
  def filter_map
    out = [] #: Array[U]
    each do |x|
      v = yield(x)
      out << v if v
    end
    out
  end

  #: () { (E) -> bool } -> [Array[E], Array[E]]
  def partition
    yes = [] #: Array[E]
    no = [] #: Array[E]
    each { |x| yield(x) ? yes << x : no << x }
    [yes, no]
  end

  #: () -> [E?, E?]
  def minmax = [min, max]

  #: (Integer) -> Array[E]
  def __min_1(n) = sort.first(n)

  #: (Integer) -> Array[E]
  def __max_1(n) = sort.reverse.first(n)

  #: () { (E, E) -> Integer } -> Array[E]
  def __sort_block = %x{
    out := &Array[E]{}
    for x := range self.Each() {
      out.s = append(out.s, x)
    }
    slices.SortStableFunc(out.s, func(a, b E) int { return int(blk(a, b)) })
    return out
  }

  #: () { (E) -> bool } -> Array[E]
  def take_while
    out = [] #: Array[E]
    each do |x|
      break unless yield(x)
      out << x
    end
    out
  end

  #: () { (E) -> bool } -> Array[E]
  def drop_while
    out = [] #: Array[E]
    dropping = true
    each do |x|
      dropping = false if dropping && !yield(x)
      out << x unless dropping
    end
    out
  end

  #: (Integer) -> Array[E]
  def drop(n)
    raise ArgumentError, "attempt to drop negative size" if n < 0
    out = [] #: Array[E]
    i = 0
    each do |x|
      out << x if i >= n
      i += 1
    end
    out
  end

  #: (Integer) { (Array[E]) -> void } -> void
  def each_slice(n)
    raise ArgumentError, "invalid slice size" if n <= 0
    cur = [] #: Array[E]
    each do |x|
      cur << x
      if cur.size == n
        yield cur
        cur = []
      end
    end
    yield cur unless cur.empty?
  end

  #: (Integer) -> Enumerator[Array[E]]
  def __each_slice_enum(n) = %x{
    if n <= 0 {
      panic(NewArgumentError(Ref(String("invalid slice size"))))
    }
    size := rbCountSize(any(self))
    return rbEnumOf(rbSlices(self.Each(), int(n)), any(self), "each_slice("+string(rbInspect(n))+")", func() *Integer {
      c := size()
      if c == nil {
        return nil
      }
      return Ref((*c + n - 1) / n)
    }, nil)
  }

  #: (Integer) { (Array[E]) -> void } -> void
  def each_cons(n)
    raise ArgumentError, "invalid size" if n <= 0
    all = to_a
    i = 0
    while i + n <= all.size
      yield(all[i, n] || [])
      i += 1
    end
  end

  #: (Integer) -> Enumerator[Array[E]]
  def __each_cons_enum(n) = %x{
    if n <= 0 {
      panic(NewArgumentError(Ref(String("invalid size"))))
    }
    size := rbCountSize(any(self))
    return rbEnumOf(rbCons(self.Each(), int(n)), any(self), "each_cons("+string(rbInspect(n))+")", func() *Integer {
      c := size()
      if c == nil {
        return nil
      }
      return Ref(max(*c-n+1, 0))
    }, nil)
  }

  #: () -> Enumerator[[E, Integer]]
  def __each_with_index_enum = %x{ return rbEnumOf(rbWithIndex(self.Each(), 0), any(self), "each_with_index", rbCountSize(any(self)), nil) }

  #: [U] (Array[U]) -> Array[[E, U?]]
  def zip(other)
    out = [] #: Array[[E, U?]]
    i = 0
    each do |x|
      out << [x, other[i]]
      i += 1
    end
    out
  end

  #: () { (E, E) -> bool } -> Array[Array[E]]
  def chunk_while = %x{
    out := &Array[*Array[E]]{}
    var cur *Array[E]
    var prev E
    for x := range self.Each() {
      if cur != nil && bool(blk(prev, x)) {
        cur.s = append(cur.s, x)
      } else {
        cur = &Array[E]{s: []E{x}}
        out.s = append(out.s, cur)
      }
      prev = x
    }
    return out
  }

  #: () { (E, E) -> bool } -> Array[Array[E]]
  def slice_when = %x{
    out := &Array[*Array[E]]{}
    var cur *Array[E]
    var prev E
    for x := range self.Each() {
      if cur != nil && !bool(blk(prev, x)) {
        cur.s = append(cur.s, x)
      } else {
        cur = &Array[E]{s: []E{x}}
        out.s = append(out.s, cur)
      }
      prev = x
    }
    return out
  }

  #: [K] () { (E) -> K } -> Array[E]
  def uniq_by
    seen = {} #: Hash[K, bool]
    out = [] #: Array[E]
    each do |x|
      k = yield(x)
      next if seen.key?(k)
      seen[k] = true
      out << x
    end
    out
  end

  #: () { (E) -> bool } -> Array[E]
  def find_all
    out = [] #: Array[E]
    each { |x| out << x if yield(x) }
    out
  end

  #: () { (E) -> bool } -> Array[E]
  def filter
    out = [] #: Array[E]
    each { |x| out << x if yield(x) }
    out
  end

  #: (Integer) { (E, Integer) -> void } -> void
  def each_with_index_from(offset)
    i = offset
    each do |x|
      yield(x, i)
      i += 1
    end
  end

  # Eager: an Array of every combination, not an Enumerator.
  #: (Integer) -> Array[Array[E]]
  def combination(k) = %x{ return rbCombinations(slices.Collect(self.Each()), int(k)) }

  # Eager, in MRI's order.
  #: (Integer) -> Array[Array[E]]
  def __permutation_1(k) = %x{ return rbPermutations(slices.Collect(self.Each()), int(k)) }

  #: () -> Array[Array[E]]
  def permutation = %x{
    all := slices.Collect(self.Each())
    return rbPermutations(all, len(all))
  }

  # Array's values_at (Hash has its own): a Go instantiation cycle forbids
  # an Array[E] method building an Array[E?], so it lives here.
  #: (*Integer) -> Array[E?]
  def values_at(*idx)
    all = to_a
    idx.map { |i| all[i] }
  end

  #: () { (E) -> bool } -> bool
  def one?
    n = 0
    each do |x|
      n += 1 if yield(x)
      return false if n > 1
    end
    n == 1
  end

  #: (E) -> Integer?
  def find_index(v)
    i = 0
    each do |x|
      return i if x == v
      i += 1
    end
    nil
  end

  #: () { (E) -> bool } -> Integer?
  def __find_index_block
    i = 0
    each do |x|
      return i if yield(x)
      i += 1
    end
    nil
  end

  # One pass, so the block runs once per element as in MRI.
  #: [K] () { (E) -> K } -> [E?, E?]
  def minmax_by = %x{
    var lo, hi *E
    var loK, hiK K
    for x := range self.Each() {
      k := blk(x)
      if lo == nil || rbCmp(loK, k) > 0 {
        lo, loK = &x, k
      }
      if hi == nil || rbCmp(hiK, k) < 0 {
        hi, hiK = &x, k
      }
    }
    return Tuple2[*E, *E]{lo, hi}
  }

  #: [U] () { (E) -> Array[U] } -> Array[U]
  def collect_concat(&block) = flat_map(&block)

  #: [A] (A) { (E, A) -> void } -> A
  def with_object(memo)
    each { |x| yield(x, memo) }
    memo
  end

  #: (?Integer?) { (E) -> void } -> void
  def cycle(n = nil)
    all = to_a
    return if all.empty?
    if n
      n.times { all.each { |x| yield x } }
      return
    end
    while true
      all.each { |x| yield x }
    end
  end

  #: () { (E) -> bool } -> Array[Array[E]]
  def slice_before = %x{
    out := &Array[*Array[E]]{}
    var cur *Array[E]
    for x := range self.Each() {
      if cur == nil || bool(blk(x)) {
        cur = &Array[E]{}
        out.s = append(out.s, cur)
      }
      cur.s = append(cur.s, x)
    }
    return out
  }

  #: () { (E) -> bool } -> Array[Array[E]]
  def slice_after = %x{
    out := &Array[*Array[E]]{}
    var cur *Array[E]
    for x := range self.Each() {
      if cur == nil {
        cur = &Array[E]{}
        out.s = append(out.s, cur)
      }
      cur.s = append(cur.s, x)
      if bool(blk(x)) {
        cur = nil
      }
    }
    return out
  }

  # Consecutive runs by key; a nil key drops the element, as in MRI.
  #: [K] () { (E) -> K? } -> Array[[K, Array[E]]]
  def chunk = %x{
    out := &Array[Tuple2[K, *Array[E]]]{}
    var cur *Array[E]
    var last K
    for x := range self.Each() {
      k := blk(x)
      switch {
      case k == nil:
        cur = nil
      case cur != nil && bool(rbEq(last, *k)):
        cur.s = append(cur.s, x)
      default:
        cur, last = &Array[E]{s: []E{x}}, *k
        out.s = append(out.s, Tuple2[K, *Array[E]]{*k, cur})
      }
    }
    return out
  }

  #: (Integer) -> Array[Array[E]]
  def repeated_combination(k) = %x{ return rbRepeated(slices.Collect(self.Each()), int(k), true) }

  #: (Integer) -> Array[Array[E]]
  def repeated_permutation(k) = %x{ return rbRepeated(slices.Collect(self.Each()), int(k), false) }
end
