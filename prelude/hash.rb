# prelude/hash.rb
# rbs_inline: enabled
#
# Hash: insertion-ordered, a struct over a Go map plus a key list.

# Insertion-ordered, like Ruby. Deletion is O(n) (docs/design.md decision 1).
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
    rbFrozenCheck(self)
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
    raise KeyError.__for("key not found: #{k.inspect}", self, k)
  end

  #: (K, V?) -> V?
  def __fetch_opt(k, fallback) = %x{
    if v, ok := self.rbGet(k); ok {
      return &v
    }
    return fallback
  }

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
    rbFrozenCheck(self)
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
    rbFrozenCheck(self)
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

  #: [U] () { (V) -> U } -> Hash[K, U]
  def transform_values
    out = {} #: Hash[K, U]
    each { |k, v| out[k] = yield(v) }
    out
  end

  #: [U] () { (K) -> U } -> Hash[U, V]
  def transform_keys
    out = {} #: Hash[U, V]
    each { |k, v| out[yield(k)] = v }
    out
  end

  #: [A, B] () { (K, V) -> [A, B] } -> Hash[A, B]
  def __to_h_block
    out = {} #: Hash[A, B]
    each do |k, v|
      pair = yield(k, v)
      out[pair[0]] = pair[1]
    end
    out
  end

  #: () -> Hash[K, V]
  def to_h = merge({})

  # A hash pattern's view of the Hash: itself, whatever keys it asks for, as MRI's (decision 143).
  #: (Array[Symbol]?) -> Hash[K, V]
  def deconstruct_keys(_keys) = self

  #: () -> Hash[V, K]
  def invert
    out = {} #: Hash[V, K]
    each { |k, v| out[v] = k }
    out
  end

  #: (V) -> K?
  def key(value)
    each { |k, v| return k if v == value }
    nil
  end

  #: (V) -> bool
  def value?(value)
    each { |_k, v| return true if v == value }
    false
  end

  #: (V) -> bool
  def has_value?(value) = value?(value)

  #: (K) -> bool
  def member?(k) = key?(k)

  #: (*K) -> Array[V?]
  def values_at(*ks) = ks.map { |k| self[k] }

  #: (*K) -> Array[V]
  def fetch_values(*ks) = ks.map { |k| fetch(k) }

  #: (*K) -> Hash[K, V]
  def slice(*ks) = %x{
    out := NewHash[K, V]()
    for _, k := range rest_ {
      if v, ok := self.vals[k]; ok {
        Hash_Op_idxSet(out, k, v)
      }
    }
    return out
  }

  #: (*K) -> Hash[K, V]
  def except(*ks)
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v unless ks.include?(k) }
    out
  end

  #: (K, V) -> V
  def store(k, v) = self[k] = v

  #: (Hash[K, V]) -> self
  def update(other)
    other.each { |k, v| self[k] = v }
    self
  end

  #: (Hash[K, V]) -> self
  def merge!(other) = update(other)

  # The block resolves a key both hashes hold: (key, old, new) -> value.
  #: (Hash[K, V]) { (K, V, V) -> V } -> Hash[K, V]
  def __merge_block(other) = %x{
    out := NewHash[K, V]()
    for _, k := range self.keys {
      Hash_Op_idxSet(out, k, self.vals[k])
    }
    for _, k := range other.keys {
      if old, ok := out.vals[k]; ok {
        Hash_Op_idxSet(out, k, blk(k, old, other.vals[k]))
      } else {
        Hash_Op_idxSet(out, k, other.vals[k])
      }
    }
    return out
  }

  #: () { (K, V) -> bool } -> self
  def delete_if = %x{
    rbFrozenCheck(self)
    for _, k := range slices.Clone(self.keys) {
      if v, ok := self.vals[k]; ok && bool(blk(k, v)) {
        Hash_Delete(self, k)
      }
    }
    return self
  }

  #: () { (K, V) -> bool } -> self
  def keep_if = %x{
    rbFrozenCheck(self)
    for _, k := range slices.Clone(self.keys) {
      if v, ok := self.vals[k]; ok && !bool(blk(k, v)) {
        Hash_Delete(self, k)
      }
    }
    return self
  }

  #: () { (K, V) -> bool } -> self
  def reject!(&block) = delete_if(&block)

  #: () { (K, V) -> bool } -> self
  def select!(&block) = keep_if(&block)

  #: () { (K, V) -> bool } -> self
  def filter!(&block) = keep_if(&block)

  #: () { (K, V) -> bool } -> Integer
  def __count_block
    n = 0
    each { |k, v| n += 1 if yield(k, v) }
    n
  end

  #: () { (K, V) -> bool } -> bool
  def any?
    each { |k, v| return true if yield(k, v) }
    false
  end

  #: () { (K, V) -> bool } -> bool
  def all?
    each { |k, v| return false unless yield(k, v) }
    true
  end

  #: () { (K, V) -> bool } -> bool
  def none?
    each { |k, v| return false if yield(k, v) }
    true
  end

  #: () -> [K, V]?
  def first_pair
    each { |k, v| return [k, v] }
    nil
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
        return self._ToAny().Op_eq(h._ToAny())
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
      Hash_Op_idxSet(out, rbUnbox(k), rbUnbox(self.vals[k]))
    }
    return out
  }

  # Frozen by identity (rbFreeze); every mutator above checks first (decision 96).
  #: () -> self
  def freeze = %x{
    rbFreeze(self)
    return self
  }

  #: () -> bool
  def frozen? = %x{ Boolean(rbIsFrozen(self)) }

  #: () -> [K, V]?
  def shift
    pair = first_pair
    delete(pair[0]) if pair
    pair
  end

  #: (K) -> [K, V]?
  def assoc(k) = key?(k) ? [k, fetch(k)] : nil

  #: (Hash[K, V]) -> self
  def replace(other)
    pairs = other.to_a
    clear
    pairs.each { |k, v| self[k] = v }
    self
  end

  #: () -> self
  def to_hash = self

  #: (K, *untyped) -> untyped
  def dig(k, *rest) = %x{ return rbDig(self, append([]any{k}, rest_...)) }

  #: (?Integer) -> Array[untyped]
  def flatten(depth = 1)
    out = [] #: Array[untyped]
    each do |k, v|
      out << k
      out << v
    end
    depth > 1 ? out.flatten(depth - 1) : out
  end

  # Without the nil values. V keeps its type: a Hash[K, V?] result is still V?, though it holds no nil.
  #: () -> Hash[K, V]
  def compact
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v unless v.nil? }
    out
  end

  #: () -> Hash[K, V]?
  def compact!
    n = size
    delete_if { |_k, v| v.nil? }
    size == n ? nil : self
  end
end
