# prelude/hash.rb
# rbs_inline: enabled
#
# Hash: insertion-ordered, a struct over a Go map plus a key list.

%x{
  // Ruby 3.4+ prints symbol keys as labels: `a: 1`, or `"a b": 1` when the
  // symbol is not an identifier (rbIdentRe, prelude/symbol.rb).
  var rbLabelSymbol = regexp.MustCompile(`\\A` + rbIdentRe + `[?!]?\\z`)

  func rbInspectPair(k, v any) string {
    if s, ok := k.(Symbol); ok {
      if rbLabelSymbol.MatchString(string(s)) {
        return string(s) + ": " + string(rbInspect(v))
      }
      return string(rbStringInspect(string(s))) + ": " + string(rbInspect(v))
    }
    return string(rbInspect(k)) + " => " + string(rbInspect(v))
  }

  // rbKeyIndex buckets the keys of a Hash (or uniq's seen set) that match
  // by value (rbValueKey) by their hash. plain (rbPlainKey[K], computed
  // once) skips it. It is not a Hash[K, ...] because Hash's methods would
  // make uniq a Go instantiation cycle (decision 9).
  type rbKeyIndex[K comparable] struct {
    plain  bool
    byHash map[uint64][]K
  }

  // find is the map key Ruby's eql? gives k: the stored key eql? to k, or
  // k itself. Keys that do not match by value are their own map keys.
  func (idx *rbKeyIndex[K]) find(k K) (key K, h uint64, byValue bool) {
    if idx.plain {
      return k, 0, false
    }
    if h, byValue = rbValueKey(k); !byValue {
      return k, 0, false
    }
    for _, c := range idx.byHash[h] {
      if rbKeyEql(c, k) {
        return c, h, true
      }
    }
    return k, h, true
  }

  func (idx *rbKeyIndex[K]) add(k K, h uint64) {
    if idx.byHash == nil {
      idx.byHash = map[uint64][]K{}
    }
    idx.byHash[h] = append(idx.byHash[h], k)
  }

  func (idx *rbKeyIndex[K]) remove(k K, h uint64) {
    idx.byHash[h] = slices.DeleteFunc(idx.byHash[h], func(c K) bool { return c == k })
  }

  // rbGet is the value at k. It is kept out of line so that Hash#[] fits
  // Go's inlining budget: inlined, the V? it returns is not heap-allocated.
  //go:noinline
  func (self *Hash[K, V]) rbGet(k K) (V, bool) {
    if !self.idx.plain {
      k, _, _ = self.idx.find(k)
    }
    v, ok := self.vals[k]
    return v, ok
  }
}

# Insertion-ordered, like Ruby. Deletion is O(n) (README open decision 1).
# Keys match by eql?/hash: see rbKeyIndex.
# iter counts running iterators, as MRI's iter_lev: while it is non-zero []=
# of a new key raises and delete copies the key list instead of shifting the
# one being ranged over; iterators skip keys deleted under them.
# @rbs generic K
# @rbs generic V
# @go_type struct { keys []K; vals map[K]V; iter int; idx rbKeyIndex[K] }
class Hash < Object
  include Enumerable #[[K, V]]

  #: () { ([K, V]) -> void } -> void
  def each = %x{
    return func(yield func(Tuple2[K, V]) bool) {
      self.iter++
      defer func() { self.iter-- }()
      for _, k := range self.keys {
        if v, ok := self.vals[k]; ok && !yield(Tuple2[K, V]{k, v}) {
          return
        }
      }
    }
  }

  # MRI's each_pair is each: an arity-1 block gets the [k, v] pair.
  #: () { ([K, V]) -> void } -> void
  def each_pair = %x{
    return func(yield func(Tuple2[K, V]) bool) {
      self.iter++
      defer func() { self.iter-- }()
      for _, k := range self.keys {
        if v, ok := self.vals[k]; ok && !yield(Tuple2[K, V]{k, v}) {
          return
        }
      }
    }
  }

  #: () { (K) -> void } -> void
  def each_key = %x{
    return func(yield func(K) bool) {
      self.iter++
      defer func() { self.iter-- }()
      for _, k := range self.keys {
        if _, ok := self.vals[k]; ok && !yield(k) {
          return
        }
      }
    }
  }

  #: () { (V) -> void } -> void
  def each_value = %x{
    return func(yield func(V) bool) {
      self.iter++
      defer func() { self.iter-- }()
      for _, k := range self.keys {
        if v, ok := self.vals[k]; ok && !yield(v) {
          return
        }
      }
    }
  }

  # RBS core says `(K) -> V`; that is only true with a default. Be honest.
  #: (K) -> V?
  def [](k) = %x{
    v, ok := self.rbGet(k)
    if !ok {
      return nil
    }
    return &v
  }

  #: (K, V) -> V
  def []=(k, v)
    %x{
    var h uint64
    byValue := false
    if !self.idx.plain {
      k, h, byValue = self.idx.find(k)
    }
    if _, ok := self.vals[k]; !ok {
      if self.iter > 0 {
        panic(NewRuntimeError(Ref[String]("can't add a new key into hash during iteration")))
      }
      self.keys = append(self.keys, k)
      if byValue {
        self.idx.add(k, h)
      }
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
    clear(self.idx.byHash)
    return self
  }

  #: (K) -> bool
  def key?(k) = %x{
    _, ok := self.rbGet(k)
    return Boolean(ok)
  }

  #: (K) -> bool
  def include?(k) = key?(k)

  #: (K) -> bool
  def has_key?(k) = key?(k)

  #: (K) -> V?
  def delete(k) = %x{
    k, h, byValue := self.idx.find(k)
    v, ok := self.vals[k]
    if !ok {
      return nil
    }
    delete(self.vals, k)
    if byValue {
      self.idx.remove(k, h)
    }
    for i, key := range self.keys {
      if key == k {
        head := self.keys[:i]
        if self.iter > 0 {
          head = head[:i:i] // force a copy
        }
        self.keys = append(head, self.keys[i+1:]...)
        break
      }
    }
    return &v
  }

  # Unlike Enumerable#select, Ruby's Hash#select returns a Hash. MRI
  # yields (k, v), not the pair, so an arity-1 block gets the key.
  #: () { (K, V) -> bool } -> Hash[K, V]
  def select
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v if yield(k, v) }
    out
  end

  #: () { (K, V) -> bool } -> Hash[K, V]
  def filter
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v if yield(k, v) }
    out
  end

  #: () { (K, V) -> bool } -> Hash[K, V]
  def reject
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v unless yield(k, v) }
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
  def inspect = %x{
    if !rbInspectEnter(self) {
      return "{...}"
    }
    defer rbInspectLeave(self)
    parts := make([]string, 0, len(self.keys))
    for _, k := range self.keys {
      parts = append(parts, rbInspectPair(k, self.vals[k]))
    }
    return String("{" + strings.Join(parts, ", ") + "}")
  }

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Hash[K, V])
    if !ok {
      // Another instantiation ({1 => 1} == {1 => 1.0}, typed vs untyped):
      // compare the untyped views, whose == takes the branch below.
      if h, ok := other.(Hash_Any); ok {
        return self._ToAny().Eq(h._ToAny())
      }
      return false
    }
    if len(o.keys) != len(self.keys) {
      return false
    }
    for k, v := range self.vals {
      k, _, _ = o.idx.find(k)
      ov, ok := o.vals[k]
      if !ok || !bool(rbEq(v, ov)) {
        return false
      }
    }
    return true
  }

  # Like ==, but values compare by eql? too ({1 => 1} is not eql? to
  # {1 => 1.0}).
  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := other.(*Hash[K, V])
    if !ok {
      if h, ok := other.(Hash_Any); ok {
        return self._ToAny().EqlQ(h._ToAny())
      }
      return false
    }
    if len(o.keys) != len(self.keys) {
      return false
    }
    for k, v := range self.vals {
      k, _, _ = o.idx.find(k)
      ov, ok := o.vals[k]
      if !ok || !rbKeyEql(v, ov) {
        return false
      }
    }
    return true
  }

  # Order-independent, as eql? is.
  #: () -> Integer
  def hash = %x{
    h := uint64(len(self.keys))
    for k, v := range self.vals {
      h += rbKeyHash(k)*31 ^ rbKeyHash(v)
    }
    return Integer(h)
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
