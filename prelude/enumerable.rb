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

  #: (E) -> bool
  def include?(v)
    each { |x| return true if x == v }
    false
  end

  #: () -> Array[E]
  def to_a
    out = [] #: Array[E]
    each { |x| out << x }
    out
  end

  #: () -> Hash[E, Integer]
  def tally
    out = {} #: Hash[E, Integer]
    each { |x| out[x] = (out[x] || 0) + 1 }
    out
  end

  #: (Integer) -> Array[E]
  def first(n)
    out = [] #: Array[E]
    each do |x|
      break if out.size >= n
      out << x
    end
    out
  end

  #: (Integer) -> Array[E]
  def take(n) = first(n)

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
    slices.SortStableFunc(tmp, func(a, b kv) int { return int(rbCmp(a.k, b.k)) })
    out := &Array[E]{}
    for _, p := range tmp {
      *out = append(*out, p.v)
    }
    return out
  }

  #: () -> Array[E]
  def sort = %x{
    out := &Array[E]{}
    for x := range self.Each() {
      *out = append(*out, x)
    }
    slices.SortStableFunc(*out, func(a, b E) int { return int(rbCmp(a, b)) })
    return out
  }

  #: () -> E?
  def min = %x{
    var best *E
    for x := range self.Each() {
      if best == nil || rbCmp(x, *best) < 0 {
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
      if best == nil || rbCmp(x, *best) > 0 {
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
      if best == nil || rbCmp(k, bestK) < 0 {
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
      if best == nil || rbCmp(k, bestK) > 0 {
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
end
