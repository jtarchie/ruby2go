# rbs_inline: enabled

# Iteration needs Integer or String (MRI's succ); other E only compare.

# @rbs generic E
# @go_type struct { b E; e E; excl bool; endless bool; beginless bool }
class Range < Object
  include Enumerable #[E]

  #: () { (E) -> void } -> void
  def each = %x{
    return func(yield func(E) bool) { rbRangeEach(self, yield) }
  }

  #: () { (E) -> void } -> void
  def reverse_each = %x{
    return func(yield func(E) bool) {
      rbRangeNoBegin(self)
      if self.endless {
        panic(NewTypeError(Ref(String("can't iterate from " + rbClassName(self.e)))))
      }
      b, bok := any(self.b).(Integer)
      e, eok := any(self.e).(Integer)
      if !bok || !eok {
        xs := []E{}
        rbRangeEach(self, func(x E) bool { xs = append(xs, x); return true })
        for i := len(xs) - 1; i >= 0; i-- {
          if !yield(xs[i]) {
            return
          }
        }
        return
      }
      if self.excl {
        e--
      }
      for i := e; i >= b; i-- {
        if !yield(any(i).(E)) {
          return
        }
      }
    }
  }

  #: (Integer) { (E) -> void } -> void
  def step(n) = %x{
    return func(yield func(E) bool) {
      if n < 0 {
        panic(NewArgumentError(Ref(String("step can't be negative"))))
      }
      if n == 0 {
        panic(NewArgumentError(Ref(String("step can't be 0"))))
      }
      i := 0
      rbRangeEach(self, func(x E) bool {
        ok := i%int(n) != 0 || yield(x)
        i++
        return ok
      })
    }
  }

  #: () -> E
  def begin = %x{ self.b }

  #: () -> E
  def __first_0 = %x{ self.b }

  #: () -> E
  def end = %x{ self.e }

  #: () -> E
  def last = %x{
    if self.endless {
      panic(NewRangeError(Ref(String("cannot get the last element of endless range"))))
    }
    return self.e
  }

  #: () -> bool
  def exclude_end? = %x{ Boolean(self.excl) }

  #: () -> bool
  def __endless? = %x{ Boolean(self.endless) }

  #: (E) -> bool
  def cover?(v) = %x{ Boolean(rbRangeCover(self, v)) }

  #: (E) -> bool
  def include?(v) = %x{ Boolean(rbRangeCover(self, v)) }

  #: (E) -> bool
  def member?(v) = %x{ Boolean(rbRangeCover(self, v)) }

  # case/when: an incomparable value (another class, nil) is simply not covered.
  #: (untyped) -> bool
  def ===(v) = %x{
    x, ok := rbUnbox(v).(E)
    return Boolean(ok && rbRangeCover(self, x))
  }

  #: () -> Integer
  def size = %x{
    rbRangeNoBegin(self)
    b, ok := any(self.b).(Integer)
    if !ok {
      panic(NewTypeError(Ref(String("can't iterate from " + rbClassName(self.b)))))
    }
    if self.endless {
      panic(NewRangeError(Ref(String("cannot get the size of endless range (rb2go has no Infinity Integer)"))))
    }
    return rbRangeIntCount(b, any(self.e).(Integer), self.excl)
  }

  #: () -> Integer
  def count = %x{
    if b, ok := any(self.b).(Integer); ok && !self.endless && !self.beginless {
      return rbRangeIntCount(b, any(self.e).(Integer), self.excl)
    }
    n := Integer(0)
    rbRangeEach(self, func(E) bool { n++; return true })
    return n
  }

  #: () -> Integer
  def length = size

  #: () -> Integer
  def sum = %x{
    b, bok := any(self.b).(Integer)
    e, eok := any(self.e).(Integer)
    if !bok || !eok || self.endless || self.beginless {
      panic(NewTypeError(Ref(String("rb2go: Range#sum needs an Integer range"))))
    }
    if self.excl {
      e--
    }
    if e < b {
      return 0
    }
    return (e - b + 1) * (b + e) / 2
  }

  #: () -> E?
  def min = %x{
    if self.beginless {
      panic(NewRangeError(Ref(String("cannot get the minimum of beginless range"))))
    }
    if self.endless {
      return &self.b
    }
    if c := rbCmp(self.b, self.e); c > 0 || (c == 0 && self.excl) {
      return nil
    }
    return &self.b
  }

  #: () -> E?
  def max = %x{
    if self.endless {
      panic(NewRangeError(Ref(String("cannot get the maximum of endless range"))))
    }
    if c := rbCmp(self.b, self.e); !self.beginless && (c > 0 || (c == 0 && self.excl)) {
      return nil
    }
    if !self.excl {
      return &self.e
    }
    e, ok := any(self.e).(Integer)
    if !ok {
      panic(NewTypeError(Ref(String("cannot exclude non Integer end value"))))
    }
    x := any(e - 1).(E)
    return &x
  }

  #: () -> Array[E]
  def to_a = %x{
    if self.endless {
      panic(NewRangeError(Ref(String("cannot convert endless range to an array"))))
    }
    out := &Array[E]{}
    rbRangeEach(self, func(x E) bool { *out = append(*out, x); return true })
    return out
  }

  #: () -> Array[E]
  def entries = to_a

  #: () -> Array[E]
  def to_ary = to_a

  #: () -> String
  def to_s = %x{ rbRangeStr(self, rbToS) }

  #: () -> String
  def inspect = %x{ rbRangeStr(self, rbInspect) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Range[E])
    return Boolean(ok && o.excl == self.excl && o.endless == self.endless && o.beginless == self.beginless && rbKeyEql(o.b, self.b) && rbKeyEql(o.e, self.e))
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{
    h := rbHash(self.b)*31 + rbHash(self.e)
    if self.excl {
      h++
    }
    return h
  }

  # Immediate-like: literal ranges are frozen in MRI.
  #: () -> bool
  def frozen? = true

  # [start, length] within size n; nil when start is out of bounds, as MRI.
  #: (Integer) -> [Integer, Integer]?
  def __slice(n) = %x{
    b, ok := any(self.b).(Integer)
    if self.beginless {
      b, ok = 0, true
    }
    if !ok {
      panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(self.b) + " into Integer"))))
    }
    e := n
    if !self.endless {
      e = any(self.e).(Integer)
      if e < 0 {
        e += n
      }
      if !self.excl {
        e++
      }
    }
    if b < 0 {
      b += n
    }
    if b < 0 || b > n {
      return nil
    }
    e = min(e, n)
    return &Tuple2[Integer, Integer]{b, max(e-b, 0)}
  }

  #: () -> Range[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Range[any]); ok {
      return same
    }
    return &Range[any]{b: rbUnbox(self.b), e: rbUnbox(self.e), excl: self.excl, endless: self.endless, beginless: self.beginless}
  }

  #: (Range[E]) -> bool
  def overlap?(other) = %x{ Boolean(rbRangeOverlap(self, other)) }

  # Integer ranges only: the smallest value the block is true for (find-minimum mode).
  #: () { (E) -> bool } -> E?
  def bsearch = %x{ return rbRangeBsearch(self, func(x E) bool { return bool(blk(x)) }) }
end
