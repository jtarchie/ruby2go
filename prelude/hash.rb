# prelude/hash.rb
# rbs_inline: enabled
#
# Hash: insertion-ordered, a struct over a Go map plus a key list.

# Insertion-ordered, like Ruby. Deletion is O(n) (README open decision 1).
# @rbs generic K
# @rbs generic V
# @go_type struct { keys []K; vals map[K]V }
class Hash < Object
  include Enumerable #[[K, V]]

  #: () { ([K, V]) -> void } -> void
  def each = %x{
    return func(yield func(Tuple2[K, V]) bool) {
      for _, k := range self.keys {
        if !yield(Tuple2[K, V]{k, self.vals[k]}) {
          return
        }
      }
    }
  }

  #: () { (K, V) -> void } -> void
  def each_pair = %x{
    return func(yield func(K, V) bool) {
      for _, k := range self.keys {
        if !yield(k, self.vals[k]) {
          return
        }
      }
    }
  }

  #: () { (K) -> void } -> void
  def each_key = %x{
    return func(yield func(K) bool) {
      for _, k := range self.keys {
        if !yield(k) {
          return
        }
      }
    }
  }

  #: () { (V) -> void } -> void
  def each_value = %x{
    return func(yield func(V) bool) {
      for _, k := range self.keys {
        if !yield(self.vals[k]) {
          return
        }
      }
    }
  }

  # RBS core says `(K) -> V`; that is only true with a default. Be honest.
  #: (K) -> V?
  def [](k) = %x{
    v, ok := self.vals[k]
    if !ok {
      return nil
    }
    return &v
  }

  #: (K, V) -> V
  def []=(k, v)
    %x{
    if _, ok := self.vals[k]; !ok {
      self.keys = append(self.keys, k)
    }
    self.vals[k] = v
    return v}
  end

  #: (K, V) -> self
  def __set(k, v)
    self[k] = v
    self
  end

  #: (K, ?V?) -> V
  def fetch(k, default = nil)
    v = self[k]
    return v if v
    return default if default
    raise KeyError, "key not found: #{k.inspect}"
  end

  # A new hash with other's pairs laid over this one's.
  #: (Hash[K, V]) -> Hash[K, V]
  def merge(other)
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v }
    other.each { |k, v| out[k] = v }
    out
  end

  #: () -> self
  def clear = %x{
    self.keys = self.keys[:0]
    clear(self.vals)
    return self
  }

  #: (K) -> bool
  def key?(k) = %x{
    _, ok := self.vals[k]
    return Boolean(ok)
  }

  #: (K) -> bool
  def include?(k) = key?(k)

  #: (K) -> bool
  def has_key?(k) = key?(k)

  #: (K) -> V?
  def delete(k) = %x{
    v, ok := self.vals[k]
    if !ok {
      return nil
    }
    delete(self.vals, k)
    for i, key := range self.keys {
      if key == k {
        self.keys = append(self.keys[:i], self.keys[i+1:]...)
        break
      }
    }
    return &v
  }

  # Unlike Enumerable#select, Ruby's Hash#select returns a Hash.
  #: () { ([K, V]) -> bool } -> Hash[K, V]
  def select
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v if yield([k, v]) }
    out
  end

  #: () { ([K, V]) -> bool } -> Hash[K, V]
  def filter
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v if yield([k, v]) }
    out
  end

  #: () { ([K, V]) -> bool } -> Hash[K, V]
  def reject
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v unless yield([k, v]) }
    out
  end

  #: () -> Integer
  def size = %x{ Integer(len(self.keys)) }

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = %x{ len(self.keys) == 0 }

  #: () -> Array[K]
  def keys = %x{
    out := &Array[K]{}
    *out = append(*out, self.keys...)
    return out
  }

  #: () -> Array[V]
  def values = %x{
    out := &Array[V]{}
    for _, k := range self.keys {
      *out = append(*out, self.vals[k])
    }
    return out
  }

  #: () -> String
  def inspect
    return "{}" if empty?
    "{" + map { |k, v| k.inspect + " => " + v.inspect }.join(", ") + "}"
  end

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Hash[K, V])
    if !ok || len(o.keys) != len(self.keys) {
      return false
    }
    for k, v := range self.vals {
      ov, ok := o.vals[k]
      if !ok || !bool(rbEq(v, ov)) {
        return false
      }
    }
    return true
  }

  #: () -> Hash[untyped, untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Hash[any, any]); ok {
      return same
    }
    out := NewHash[any, any]()
    for _, k := range self.keys {
      out.IdxSet(k, self.vals[k])
    }
    return out
  }
end
