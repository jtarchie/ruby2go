# prelude/array.rb
# rbs_inline: enabled
#
# Array as a named Go slice, always handled as a pointer.

# Array is mutable and aliased in Ruby, so it is always handled as a pointer.
# @rbs generic E
# @go_type []E
class Array < Object
  include Enumerable #[E]

  #: () { (E) -> void } -> void
  def each = %x{
    return func(yield func(E) bool) {
      // An index loop re-reading the length, as MRI's: the block may
      // append, delete or clear. (Not `range len`: that reads it once.)
      for i := 0; ; i++ {
        if i >= len(*self) || !yield((*self)[i]) {
          return
        }
      }
    }
  }

  #: () { (E, Integer) -> void } -> void
  def each_with_index = %x{
    return func(yield func(E, Integer) bool) {
      for i := 0; ; i++ {
        if i >= len(*self) || !yield((*self)[i], Integer(i)) {
          return
        }
      }
    }
  }

  #: () { (E) -> void } -> void
  def reverse_each = %x{
    return func(yield func(E) bool) {
      for i := len(*self) - 1; i >= 0; i-- {
        if !yield((*self)[i]) {
          return
        }
      }
    }
  }

  #: (Integer) -> E?
  def [](i) = %x{
    if i < 0 {
      i += Integer(len(*self))
    }
    if i < 0 || int(i) >= len(*self) {
      return nil
    }
    return &(*self)[i]
  }

  #: (Range[Integer]) -> Array[E]?
  def __idx_range(r)
    s = r.__slice(size)
    return nil unless s
    __idx_2(s[0], s[1])
  end

  #: (Integer, Integer) -> Array[E]?
  def __idx_2(start, count) = %x{
    n := Integer(len(*self))
    if start < 0 {
      start += n
    }
    if start < 0 || start > n || count < 0 {
      return nil
    }
    end := min(start+count, n)
    out := make(Array[E], end-start)
    copy(out, (*self)[start:end])
    return Ref(&out)
  }

  #: (Range[Integer]) -> Array[E]?
  def slice(r) = self[r]

  #: (Integer, E) -> E
  def []=(i, v)
    %x{
    if i < 0 {
      i += Integer(len(*self))
    }
    for int(i) >= len(*self) {
      // nil when E holds it; a non-nilable E pads with its zero value (decision 7)
      var zero E
      *self = append(*self, zero)
    }
    (*self)[i] = v
    return v}
  end

  #: (Integer) -> E
  def fetch(i)
    v = self[i]
    return v if v
    raise IndexError, "index #{i} outside of array bounds: #{-size}...#{size}"
  end

  #: (E) -> self
  def <<(x) = %x{
    *self = append(*self, x)
    return self
  }

  #: (E) -> self
  def push(x) = self << x

  #: (E) -> self
  def append(x) = self << x

  #: () -> E?
  def pop = %x{
    if len(*self) == 0 {
      return nil
    }
    x := (*self)[len(*self)-1]
    *self = (*self)[:len(*self)-1]
    return &x
  }

  #: () -> E?
  def shift = %x{
    if len(*self) == 0 {
      return nil
    }
    x := (*self)[0]
    *self = (*self)[1:]
    return &x
  }

  #: (E) -> self
  def unshift(x) = %x{
    *self = append([]E{x}, *self...)
    return self
  }

  #: (Array[E]) -> self
  def concat(other) = %x{
    *self = append(*self, *other...)
    return self
  }

  #: (Array[E]) -> Array[E]
  def +(other) = %x{
    out := &Array[E]{}
    *out = append(append(*out, *self...), *other...)
    return out
  }

  # MRI's Array#first(-1) message; Enumerable#first raises it.
  #: () -> String
  def __negative_first = "negative array size"

  #: () -> Integer
  def size = %x{ Integer(len(*self)) }

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = %x{ len(*self) == 0 }

  #: () -> E?
  def last = self[-1]

  #: (Integer) -> Array[E]
  def __last_1(n)
    raise ArgumentError, "negative array size" if n < 0
    self[[size - n, 0].max.to_i, n] || []
  end

  #: () -> Array[E]
  def reverse = %x{
    out := &Array[E]{}
    for i := len(*self) - 1; i >= 0; i-- {
      *out = append(*out, (*self)[i])
    }
    return out
  }

  #: () -> Array[E]
  def to_a = self

  #: () -> Array[E]
  def dup = %x{
    out := &Array[E]{}
    *out = append(*out, *self...)
    return out
  }

  # By eql?/hash, as Hash keys.
  #: () -> Array[E]
  def uniq = %x{
    seen := map[E]bool{}
    idx := rbKeyIndex[E]{plain: rbPlainKey[E]()}
    out := &Array[E]{}
    for _, x := range *self {
      k, h, byValue := idx.find(x)
      if !seen[k] {
        seen[k] = true
        if byValue {
          idx.add(k, h)
        }
        *out = append(*out, x)
      }
    }
    return out
  }

  #: () -> Array[E]
  def compact = %x{
    out := &Array[E]{}
    for _, x := range *self {
      if rbUnbox(any(x)) != nil {
        *out = append(*out, x)
      }
    }
    return out
  }

  #: () -> self
  def clear = %x{
    *self = (*self)[:0]
    return self
  }

  #: (E) -> E?
  def delete(v) = %x{
    var found *E
    out := (*self)[:0]
    for _, x := range *self {
      if rbEq(x, v) {
        x := x
        found = &x
        continue
      }
      out = append(out, x)
    }
    *self = out
    return found
  }

  #: (Integer) -> E?
  def delete_at(i) = %x{
    if i < 0 {
      i += Integer(len(*self))
    }
    if i < 0 || int(i) >= len(*self) {
      return nil
    }
    x := (*self)[i]
    *self = append((*self)[:i], (*self)[i+1:]...)
    return &x
  }

  #: (?String) -> String
  def join(sep = "") = %x{
    parts := make([]string, len(*self))
    for i, x := range *self {
      // ponytail: a self-containing array overflows here, MRI raises ArgumentError; add a visited set.
      if a, ok := any(x).(Array_Any); ok {
        parts[i] = string(a._ToAny().Join(sep))
        continue
      }
      parts[i] = string(rbToS(x))
    }
    if len(parts) == 1 { // Join would return the element itself
      return rbStrClone(String(parts[0]))
    }
    return String(strings.Join(parts, string(sep)))
  }

  #: () -> String
  def inspect = %x{
    if !rbInspectEnter(self) {
      return "[...]"
    }
    defer rbInspectLeave(self)
    parts := make([]string, len(*self))
    for i, x := range *self {
      parts[i] = string(rbInspect(x))
    }
    return String("[" + strings.Join(parts, ", ") + "]")
  }

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Array[E])
    if !ok {
      // Another instantiation ([1] == [1.0], typed vs untyped): compare
      // the untyped views, whose == takes the branch below.
      if a, ok := other.(Array_Any); ok {
        return self._ToAny().Op_eq(a._ToAny())
      }
      return false
    }
    if len(*o) != len(*self) {
      return false
    }
    for i, x := range *self {
      if !rbEq(x, (*o)[i]) {
        return false
      }
    }
    return true
  }

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := other.(*Array[E])
    if !ok {
      if a, ok := other.(Array_Any); ok {
        return self._ToAny().EqlQ(a._ToAny())
      }
      return false
    }
    if len(*o) != len(*self) {
      return false
    }
    for i, x := range *self {
      if !rbKeyEql(x, (*o)[i]) {
        return false
      }
    }
    return true
  }

  #: () -> Integer
  def hash = %x{
    h := uint64(len(*self))
    for _, x := range *self {
      h = h*31 + rbKeyHash(x)
    }
    return Integer(h)
  }

  #: (Array[E]) -> Integer
  def <=>(other) = %x{
    n := min(len(*self), len(*other))
    for i := range n {
      if c := rbCmp((*self)[i], (*other)[i]); c != 0 {
        return c
      }
    }
    return Integer(len(*self) - len(*other))
  }

  #: () -> Array[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Array[any]); ok {
      return same // already untyped: share it, so writes are seen
    }
    out := &Array[any]{}
    for _, x := range *self {
      *out = append(*out, rbUnbox(x))
    }
    return out
  }
end
