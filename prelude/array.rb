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
      for _, x := range *self {
        if !yield(x) {
          return
        }
      }
    }
  }

  #: () { (E, Integer) -> void } -> void
  def each_with_index = %x{
    return func(yield func(E, Integer) bool) {
      for i, x := range *self {
        if !yield(x, Integer(i)) {
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

  #: (Integer, E) -> E
  def []=(i, v)
    %x{
    if i < 0 {
      i += Integer(len(*self))
    }
    for int(i) >= len(*self) {
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

  #: () -> Array[E]
  def reverse = %x{
    out := &Array[E]{}
    for i := len(*self) - 1; i >= 0; i-- {
      *out = append(*out, (*self)[i])
    }
    return out
  }

  #: () -> Array[E]
  def dup = %x{
    out := &Array[E]{}
    *out = append(*out, *self...)
    return out
  }

  #: () -> Array[E]
  def uniq = %x{
    seen := map[E]bool{}
    out := &Array[E]{}
    for _, x := range *self {
      if !seen[x] {
        seen[x] = true
        *out = append(*out, x)
      }
    }
    return out
  }

  #: () -> Array[E]
  def compact = %x{
    out := &Array[E]{}
    for _, x := range *self {
      if any(x) != nil {
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
      parts[i] = string(rbToS(x))
    }
    return String(strings.Join(parts, string(sep)))
  }

  #: () -> String
  def inspect = "[" + map { |x| x.inspect }.join(", ") + "]"

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Array[E])
    if !ok || len(*o) != len(*self) {
      return false
    }
    for i, x := range *self {
      if !rbEq(x, (*o)[i]) {
        return false
      }
    }
    return true
  }

  #: () -> Array[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Array[any]); ok {
      return same // already untyped: share it, so writes are seen
    }
    out := &Array[any]{}
    for _, x := range *self {
      *out = append(*out, x)
    }
    return out
  }
end
