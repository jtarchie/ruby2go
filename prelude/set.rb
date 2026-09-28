# rbs_inline: enabled

# Core in Ruby 4; insertion-ordered like the Hash it wraps, and keyed by eql?/hash the same way.
# @rbs generic E
# @go_type struct { h *Hash[E, Boolean] }
class Set < Object
  include Enumerable #[E]

  #: [X] (Array[X]) -> Set[X]
  def self.new(xs) = %x{
    s := NewSet[X]()
    for _, x := range *xs {
      s.Add(x)
    }
    return s
  }

  #: [X] (*X) -> Set[X]
  def self.[](*xs) = Set.new(xs)

  #: () { (E) -> void } -> void
  def each = %x{
    return func(yield func(E) bool) {
      for k := range self.h.EachKey() {
        if !yield(k) {
          return
        }
      }
    }
  }

  #: (E) -> self
  def add(x) = %x{
    self.h.Op_idxSet(x, true)
    return self
  }

  #: (E) -> self
  def <<(x) = add(x)

  #: (E) -> Set[E]?
  def add?(x)
    return nil if include?(x)
    add(x)
  end

  #: (E) -> self
  def delete(x) = %x{
    self.h.Delete(x)
    return self
  }

  #: (E) -> Set[E]?
  def delete?(x)
    return nil unless include?(x)
    delete(x)
  end

  #: (E) -> bool
  def include?(x) = %x{ self.h.KeyQ(x) }

  #: (E) -> bool
  def member?(x) = include?(x)

  #: (E) -> bool
  def ===(x) = include?(x)

  #: () -> Integer
  def size = %x{ self.h.Size() }

  #: () -> Integer
  def length = size

  #: () -> Integer
  def count = size

  #: () -> bool
  def empty? = %x{ self.h.EmptyQ() }

  #: () -> self
  def clear = %x{
    self.h.Clear()
    return self
  }

  #: () -> Array[E]
  def to_a = %x{ self.h.Keys() }

  #: () -> Set[E]
  def dup = Set.new(to_a)

  #: () -> Set[E]
  def to_set = self

  #: (Set[E]) -> self
  def merge(xs)
    xs.each { |x| add(x) }
    self
  end

  #: (Array[E]) -> self
  def __merge_array(xs)
    xs.each { |x| add(x) }
    self
  end

  #: (Set[E]) -> Set[E]
  def |(o) = dup.merge(o)

  #: (Set[E]) -> Set[E]
  def union(o) = self | o

  #: (Set[E]) -> Set[E]
  def +(o) = self | o

  #: (Set[E]) -> Set[E]
  def &(o) = Set.new(to_a.select { |x| o.include?(x) })

  #: (Set[E]) -> Set[E]
  def intersection(o) = self & o

  #: (Set[E]) -> Set[E]
  def -(o) = Set.new(to_a.reject { |x| o.include?(x) })

  #: (Set[E]) -> Set[E]
  def difference(o) = self - o

  #: (Set[E]) -> Set[E]
  def ^(o) = (self | o) - (self & o)

  #: (Set[E]) -> bool
  def subset?(o) = size <= o.size && all? { |x| o.include?(x) }

  #: (Set[E]) -> bool
  def <=(o) = subset?(o)

  #: (Set[E]) -> bool
  def superset?(o) = o.subset?(self)

  #: (Set[E]) -> bool
  def >=(o) = superset?(o)

  #: (Set[E]) -> bool
  def proper_subset?(o) = size < o.size && subset?(o)

  #: (Set[E]) -> bool
  def <(o) = proper_subset?(o)

  #: (Set[E]) -> bool
  def proper_superset?(o) = o.proper_subset?(self)

  #: (Set[E]) -> bool
  def >(o) = proper_superset?(o)

  #: (Set[E]) -> bool
  def disjoint?(o) = none? { |x| o.include?(x) }

  #: (Set[E]) -> bool
  def intersect?(o) = !disjoint?(o)

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Set[E])
    if !ok {
      if a, isSet := other.(Set_Any); isSet {
        return self._ToAny().Op_eq(a._ToAny())
      }
      return false
    }
    return Boolean(o.Size() == self.Size() && bool(self.SubsetQ(o)))
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{
    var h Integer
    for k := range self.h.EachKey() {
      h += rbHash(k) // order-independent, as == is
    }
    return h
  }

  #: () -> String
  def inspect = "Set[#{to_a.map(&:inspect).join(", ")}]"

  #: () -> String
  def to_s = inspect

  #: () -> Set[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Set[any]); ok {
      return same
    }
    out := NewSet[any]()
    for k := range self.h.EachKey() {
      out.Add(rbUnbox(k))
    }
    return out
  }
end
