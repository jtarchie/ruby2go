# prelude/array.rb
# rbs_inline: enabled
#
# Array as a Go struct holding its slice and its frozen flag (decision 96), always handled as a pointer.

# Array is mutable and aliased in Ruby, so it is always handled as a pointer.
# @rbs generic E
# @go_type struct { s []E; frozen atomic.Bool }
class Array < Object
  include Enumerable #[E]

  # Every slot holds the same v, as MRI's (mutating a shared Array element shows in all).
  #: [X] (Integer, X) -> Array[X]
  def self.new(n, v) = %x{
    if n < 0 {
      panic(NewArgumentError(Ref(String("negative array size"))))
    }
    out := &Array[X]{s: make([]X, n)}
    for i := range out.s {
      out.s[i] = v
    }
    return out
  }

  #: [X] (Integer) { (Integer) -> X } -> Array[X]
  def self.__new_1(n)
    raise ArgumentError, "negative array size" if n < 0
    out = [] #: Array[X]
    n.times { |i| out << yield(i) }
    out
  end

  #: () { (E) -> void } -> void
  def each = %x{
    return func(yield func(E) bool) {
      // An index loop re-reading the length, as MRI's: the block may
      // append, delete or clear. (Not `range len`: that reads it once.)
      for i := 0; ; i++ {
        if i >= len(self.s) || !yield(self.s[i]) {
          return
        }
      }
    }
  }

  #: (E) -> Integer?
  def index(v)
    each_with_index { |x, i| return i if x == v }
    nil
  end

  #: () { (E) -> bool } -> Integer?
  def __index_block
    each_with_index { |x, i| return i if yield(x) }
    nil
  end

  #: (E) -> Integer?
  def find_index(v) = index(v)

  #: () { (E) -> bool } -> Integer?
  def __find_index_block
    each_with_index { |x, i| return i if yield(x) }
    nil
  end

  #: (E) -> Integer?
  def rindex(v)
    i = size - 1
    while i >= 0
      return i if self[i] == v
      i -= 1
    end
    nil
  end

  #: () { (E, Integer) -> void } -> void
  def each_with_index = %x{
    return func(yield func(E, Integer) bool) {
      for i := 0; ; i++ {
        if i >= len(self.s) || !yield(self.s[i], Integer(i)) {
          return
        }
      }
    }
  }

  #: () { (E) -> void } -> void
  def reverse_each = %x{
    return func(yield func(E) bool) {
      for i := len(self.s) - 1; i >= 0; i-- {
        if !yield(self.s[i]) {
          return
        }
      }
    }
  }

  #: (Integer) -> E?
  def [](i) = %x{
    if i < 0 {
      i += Integer(len(self.s))
    }
    if i < 0 || int(i) >= len(self.s) {
      return nil
    }
    return &self.s[i]
  }

  #: (Range[Integer]) -> Array[E]?
  def __idx_range(r)
    s = r.__slice(size)
    return nil unless s
    __idx_2(s[0], s[1])
  end

  #: (Integer, Integer) -> Array[E]?
  def __idx_2(start, count) = %x{
    n := Integer(len(self.s))
    if start < 0 {
      start += n
    }
    if start < 0 || start > n || count < 0 {
      return nil
    }
    end := min(start+count, n)
    out := &Array[E]{s: make([]E, end-start)}
    copy(out.s, self.s[start:end])
    return Ref(out)
  }

  #: (Range[Integer]) -> Array[E]?
  def slice(r) = self[r]

  #: (Integer, E) -> E
  def []=(i, v)
    %x{
    self.rbCheckFrozen()
    if i < 0 {
      if -i > Integer(len(self.s)) {
        panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", i, len(self.s))))))
      }
      i += Integer(len(self.s))
    }
    for int(i) >= len(self.s) {
      // nil when E holds it; a non-nilable E pads with its zero value (decision 7)
      var zero E
      self.s = append(self.s, zero)
    }
    self.s[i] = v
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
    self.rbCheckFrozen()
    self.s = append(self.s, x)
    return self
  }

  #: (*E) -> self
  def push(*xs) = %x{
    self.rbCheckFrozen()
    self.s = append(self.s, rest_...)
    return self
  }

  #: (*E) -> self
  def append(*xs) = push(*xs)

  #: () -> E?
  def pop = %x{
    self.rbCheckFrozen()
    if len(self.s) == 0 {
      return nil
    }
    x := self.s[len(self.s)-1]
    self.s = self.s[:len(self.s)-1]
    return &x
  }

  #: () -> E?
  def shift = %x{
    self.rbCheckFrozen()
    if len(self.s) == 0 {
      return nil
    }
    x := self.s[0]
    self.s = self.s[1:]
    return &x
  }

  #: (E) -> self
  def unshift(x) = %x{
    self.rbCheckFrozen()
    self.s = append([]E{x}, self.s...)
    return self
  }

  #: (Array[E]) -> self
  def concat(other) = %x{
    self.rbCheckFrozen()
    self.s = append(self.s, other.s...)
    return self
  }

  #: (Array[E]) -> Array[E]
  def +(other) = %x{
    out := &Array[E]{}
    out.s = append(append(out.s, self.s...), other.s...)
    return out
  }

  # Set operations match elements by eql?/hash, as MRI's.
  #: (Array[E]) -> Array[E]
  def -(other) = %x{
    drop := rbNewKeySet(other.s)
    out := &Array[E]{}
    for _, x := range self.s {
      if !drop.has(x) {
        out.s = append(out.s, x)
      }
    }
    return out
  }

  #: (Array[E]) -> Array[E]
  def difference(other) = self - other

  # Every directive but the C pointers P and p (decision 138).
  #: (String) -> String
  def pack(format) = %x{
    items := make([]any, len(self.s))
    for i, x := range self.s {
      items[i] = x
    }
    return String(rbPack(items, string(format)))
  }

  #: (Array[E]) -> Array[E]
  def &(other) = %x{
    keep := rbNewKeySet(other.s)
    seen := rbNewKeySet[E](nil)
    out := &Array[E]{}
    for _, x := range self.s {
      if keep.has(x) && !seen.has(x) {
        seen.put(x)
        out.s = append(out.s, x)
      }
    }
    return out
  }

  #: (Array[E]) -> Array[E]
  def intersection(other) = self & other

  #: (Array[E]) -> bool
  def intersect?(other) = !(self & other).empty?

  #: (Array[E]) -> Array[E]
  def |(other) = (self + other).uniq

  #: (Array[E]) -> Array[E]
  def union(other) = self | other

  #: (?Integer) -> Array[E]
  def rotate(n = 1)
    return [] if empty?
    k = n % size
    (self[k..] || []) + (self[0, k] || [])
  end


  #: [U] (Array[U]) -> Array[[E, U]]
  def product(other)
    out = [] #: Array[[E, U]]
    each { |x| other.each { |y| out << [x, y] } }
    out
  end

  # Find-minimum mode: the first element for which the block is true.
  #: () { (E) -> bool } -> E?
  def bsearch = %x{
    i := sort.Search(len(self.s), func(i int) bool { return bool(blk(self.s[i])) })
    if i == len(self.s) {
      return nil
    }
    return &self.s[i]
  }

  #: () -> self
  def sort! = %x{
    self.rbCheckFrozen()
    slices.SortStableFunc(self.s, func(a, b E) int { return -int(rbCmp(b, a)) }) // (earlier, later), as MRI's failure names them
    return self
  }

  #: [K] () { (E) -> K } -> self
  def sort_by! = %x{
    self.rbCheckFrozen()
    keys := make(map[int]K, len(self.s))
    idx := make([]int, len(self.s))
    for i, x := range self.s {
      idx[i], keys[i] = i, blk(x)
    }
    slices.SortStableFunc(idx, func(a, b int) int { return -int(rbCmp(keys[b], keys[a])) })
    out := &Array[E]{s: make([]E, len(self.s))}
    for i, j := range idx {
      out.s[i] = self.s[j]
    }
    self.s = out.s
    return self
  }

  #: () { (E) -> E } -> self
  def map! = %x{
    self.rbCheckFrozen()
    for i, x := range self.s {
      self.s[i] = blk(x)
    }
    return self
  }

  #: () { (E) -> E } -> self
  def collect!(&block) = map!(&block)

  #: () { (E) -> bool } -> self
  def keep_if = %x{
    self.rbCheckFrozen()
    out := self.s[:0]
    for _, x := range self.s {
      if bool(blk(x)) {
        out = append(out, x)
      }
    }
    clear(self.s[len(out):])
    self.s = out
    return self
  }

  #: () { (E) -> bool } -> self
  def delete_if = %x{
    self.rbCheckFrozen()
    out := self.s[:0]
    for _, x := range self.s {
      if !bool(blk(x)) {
        out = append(out, x)
      }
    }
    clear(self.s[len(out):])
    self.s = out
    return self
  }

  # nil when nothing was removed, as MRI's.
  #: () { (E) -> bool } -> Array[E]?
  def select!(&block)
    n = size
    keep_if(&block)
    size == n ? nil : self
  end

  #: () { (E) -> bool } -> Array[E]?
  def filter!(&block)
    n = size
    keep_if(&block)
    size == n ? nil : self
  end

  #: () { (E) -> bool } -> Array[E]?
  def reject!(&block)
    n = size
    delete_if(&block)
    size == n ? nil : self
  end

  #: () -> Array[E]?
  def uniq! = %x{
    self.rbCheckFrozen()
    u := Array_Uniq(self)
    if len(u.s) == len(self.s) {
      return nil
    }
    self.s = u.s
    return &self
  }

  #: () -> self
  def reverse! = %x{
    self.rbCheckFrozen()
    slices.Reverse(self.s)
    return self
  }

  #: (Integer, *E) -> self
  def insert(i, *objs) = %x{
    self.rbCheckFrozen()
    n := int(i)
    if n < 0 {
      n += len(self.s) + 1
    }
    if n < 0 {
      panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", int(i), len(self.s)+1)))))
    }
    for len(self.s) < n {
      var zero E
      self.s = append(self.s, zero)
    }
    self.s = slices.Insert(self.s, n, rest_...)
    return self
  }

  #: (E) -> self
  def fill(v) = %x{
    self.rbCheckFrozen()
    for i := range self.s {
      self.s[i] = v
    }
    return self
  }

  #: () { (Integer) -> void } -> void
  def each_index = %x{
    return func(yield func(Integer) bool) {
      for i := 0; ; i++ { // not range len: the block may grow the array
        if i >= len(self.s) || !yield(Integer(i)) {
          return
        }
      }
    }
  }

  #: () -> Enumerator[Integer]
  def __each_index_enum = %x{ return rbEnumOf(Array_EachIndex(self), any(self), "each_index", func() *Integer { n := Integer(len(self.s)); return &n }, nil) }

  #: () -> Enumerator[E]
  def __each_enum = %x{ return rbEnumOf(Array_Each(self), any(self), "each", func() *Integer { n := Integer(len(self.s)); return &n }, nil) }

  #: () -> Enumerator::Map[E]
  def __map_enum = Enumerator::Map.new(self)

  #: () -> Enumerator::Select[E]
  def __select_enum = Enumerator::Select.new(self, false, "select")

  #: () -> Enumerator::Select[E]
  def __filter_enum = Enumerator::Select.new(self, false, "filter")

  #: () -> Enumerator::Select[E]
  def __reject_enum = Enumerator::Select.new(self, true, "reject")

  # MRI's Array#first(-1) message; Enumerable#first raises it.
  #: () -> String
  def __negative_first = "negative array size"

  #: () -> Integer
  def size = %x{ Integer(len(self.s)) }

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = %x{ len(self.s) == 0 }

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
    for i := len(self.s) - 1; i >= 0; i-- {
      out.s = append(out.s, self.s[i])
    }
    return out
  }

  #: () -> Array[E]
  def to_a = self

  # An array pattern's view of the Array: itself, as MRI's (decision 143).
  #: () -> Array[E]
  def deconstruct = self

  #: () -> Array[E]
  def dup = %x{
    out := &Array[E]{}
    out.s = append(out.s, self.s...)
    return out
  }

  # By eql?/hash, as Hash keys.
  #: () -> Array[E]
  def uniq = %x{
    seen := map[E]bool{}
    idx := rbKeyIndex[E]{plain: rbPlainKey[E]()}
    out := &Array[E]{}
    for _, x := range self.s {
      k, h, byValue := idx.find(x)
      if !seen[k] {
        seen[k] = true
        if byValue {
          idx.add(k, h)
        }
        out.s = append(out.s, x)
      }
    }
    return out
  }

  #: () -> Array[E]
  def compact = %x{
    out := &Array[E]{}
    for _, x := range self.s {
      if rbUnbox(any(x)) != nil {
        out.s = append(out.s, x)
      }
    }
    return out
  }

  #: () -> self
  def clear = %x{
    self.rbCheckFrozen()
    self.s = self.s[:0]
    return self
  }

  #: (E) -> E?
  def delete(v) = %x{
    self.rbCheckFrozen()
    var found *E
    out := self.s[:0]
    for _, x := range self.s {
      if rbEq(x, v) {
        x := x
        found = &x
        continue
      }
      out = append(out, x)
    }
    self.s = out
    return found
  }

  #: (Integer) -> E?
  def delete_at(i) = %x{
    self.rbCheckFrozen()
    if i < 0 {
      i += Integer(len(self.s))
    }
    if i < 0 || int(i) >= len(self.s) {
      return nil
    }
    x := self.s[i]
    self.s = append(self.s[:i], self.s[i+1:]...)
    return &x
  }

  # `xs * n` repeats; `xs * ","` is join (decision 12's twin by argument class).
  #: (Integer) -> Array[E]
  def *(n)
    raise ArgumentError, "negative argument" if n < 0
    out = [] #: Array[E]
    n.times { out.concat(self) }
    out
  end

  #: (String) -> String
  def __mul_string(sep) = join(sep)

  #: (?String) -> String
  def join(sep = "") = %x{
    parts := make([]string, len(self.s))
    for i, x := range self.s {
      // ponytail: a self-containing array overflows here, MRI raises ArgumentError; add a visited set.
      if a, ok := any(x).(Array_Any); ok {
        parts[i] = string(Array_Join(a._ToAny(), sep))
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
  def inspect = _inspect_rec(nil)

  # inspect inside another container's: seen (an rbSeen) is passed down,
  # never shared between threads.
  #: (untyped) -> String
  def _inspect_rec(seen) = %x{
    s := rbSeenOf(seen)
    if !s.enter(self, nil) {
      return "[...]"
    }
    defer s.leave(self, nil)
    parts := make([]string, len(self.s))
    for i, x := range self.s {
      parts[i] = string(rbInspectIn(x, s))
    }
    return String("[" + strings.Join(parts, ", ") + "]")
  }

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = _eq_rec(other, nil)

  # == with the pairs already being compared (seen, an rbSeen), made only
  # once a nested container is reached: a flat Array allocates nothing.
  #: (untyped, untyped) -> bool
  def _eq_rec(other, seen) = %x{
    o, ok := other.(*Array[E])
    if !ok {
      // Another instantiation ([1] == [1.0], typed vs untyped): compare
      // the untyped views, whose == takes the branch below.
      if a, ok := other.(Array_Any); ok {
        return self._ToAny().Op_eq(a._ToAny())
      }
      return false
    }
    if len(o.s) != len(self.s) {
      return false
    }
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, o) {
        return true // a pair already being compared: equal, as MRI's recursive ==
      }
      defer s.leave(self, o)
    }
    for i, x := range self.s {
      if r, ok := any(x).(rbEqRec); ok {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, o)
        }
        if !r._EqRec(o.s[i], s) {
          return false
        }
      } else if !rbEq(x, o.s[i]) {
        return false
      }
    }
    return true
  }

  #: (untyped) -> bool
  def eql?(other) = _eql_rec(other, nil)

  #: (untyped, untyped) -> bool
  def _eql_rec(other, seen) = %x{
    o, ok := other.(*Array[E])
    if !ok {
      if a, ok := other.(Array_Any); ok {
        return self._ToAny().EqlQ(a._ToAny())
      }
      return false
    }
    if len(o.s) != len(self.s) {
      return false
    }
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, o) {
        return true
      }
      defer s.leave(self, o)
    }
    for i, x := range self.s {
      if r, ok := any(x).(rbEqlRec); ok {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, o)
        }
        if !r._EqlRec(o.s[i], s) {
          return false
        }
      } else if !rbKeyEql(x, o.s[i]) {
        return false
      }
    }
    return true
  }

  #: () -> Integer
  def hash = _hash_rec(nil)

  #: (untyped) -> Integer
  def _hash_rec(seen) = %x{
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, nil) {
        return 0 // inside itself: a constant, so the hash is finite and stable
      }
      defer s.leave(self, nil)
    }
    h := uint64(len(self.s))
    for _, x := range self.s {
      if r, ok := any(x).(rbHashRec); ok {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, nil)
        }
        h = h*31 + uint64(r._HashRec(s)) //nolint:gosec // a hash, not arithmetic
        continue
      }
      h = h*31 + rbKeyHash(x)
    }
    return Integer(h)
  }

  #: (Array[E]) -> Integer
  def <=>(other) = %x{
    n := min(len(self.s), len(other.s))
    for i := range n {
      if c := rbCmp(self.s[i], other.s[i]); c != 0 {
        return c
      }
    }
    return Integer(len(self.s) - len(other.s))
  }

  #: () -> Array[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Array[any]); ok {
      return same // already untyped: share it, so writes are seen
    }
    out := &Array[any]{}
    for _, x := range self.s {
      out.s = append(out.s, rbUnbox(x))
    }
    return out
  }

  # flatten on elements that are not Arrays: a copy, or, for untyped
  # elements, MRI's dynamic splice. Typed nesting goes to the @self forms
  # below (decision 92).
  #: (?Integer) -> Array[E]
  def flatten(depth = -1) = %x{
    if a, ok := any(self).(*Array[any]); ok {
      out := &Array[any]{}
      rbFlattenInto(out, a.s, int(depth))
      return any(out).(*Array[E])
    }
    return &Array[E]{s: slices.Clone(self.s)}
  }

  # Pairs are Arrays to Ruby: [[:a, 1]].flatten splices them.
  # @self Array[[A, B]]
  # @rbs [A, B] (?Integer) -> Array[untyped]
  def __flatten_pairs(depth = -1) = %x{
    xs := make([]any, len(self.s))
    for i, t := range self.s {
      xs[i] = t
    }
    out := &Array[any]{}
    rbFlattenInto(out, xs, int(depth))
    return out
  }

  # @self Array[Array[Array[U]]]
  # @rbs [U] () -> Array[U]
  def __flatten_nested3 = %x{
    rows := make([]*Array[U], 0, len(self.s))
    for _, r := range self.s {
      rows = append(rows, r.s...)
    }
    return rbFlattenRows(rows, -1)
  }

  # @self Array[Array[U]]
  # @rbs [U] (?Integer) -> Array[U]
  def __flatten_nested(depth = -1) = %x{
    if depth == 0 { // the result would be Array[Array[U]], not this signature's Array[U]
      panic(NewNotImplementedError(Ref(String("rb2go: flatten(0) of nested Arrays; use dup"))))
    }
    return rbFlattenRows(self.s, int(depth))
  }

  # @self Array[[K, V]]
  # @rbs [K, V] () -> Hash[K, V]
  def to_h
    out = {} #: Hash[K, V]
    each { |pair| out[pair[0]] = pair[1] }
    out
  end

  # Pairs as two-element Arrays: the length is checked as MRI does.
  # @self Array[Array[U]]
  # @rbs [U] () -> Hash[U, U]
  def __to_h_arrays
    out = {} #: Hash[U, U]
    each_with_index do |r, i|
      raise ArgumentError, "wrong array length at #{i} (expected 2, was #{r.size})" unless r.size == 2
      out[r.fetch(0)] = r.fetch(1)
    end
    out
  end

  # @rbs [K, V] () { (E) -> [K, V] } -> Hash[K, V]
  def __to_h_block
    out = {} #: Hash[K, V]
    each do |x|
      pair = yield(x)
      out[pair[0]] = pair[1]
    end
    out
  end

  # Rows of different lengths raise IndexError, as in MRI.
  # @self Array[Array[U]]
  # @rbs [U] () -> Array[Array[U]]
  def transpose = %x{
    out := &Array[*Array[U]]{}
    if len(self.s) == 0 {
      return out
    }
    n := len(self.s[0].s)
    for _, r := range self.s {
      if len(r.s) != n {
        panic(NewIndexError(Ref(String(fmt.Sprintf("element size differs (%d should be %d)", len(r.s), n)))))
      }
    }
    for j := range n {
      col := &Array[U]{s: make([]U, len(self.s))}
      for i, r := range self.s {
        col.s[i] = r.s[j]
      }
      out.s = append(out.s, col)
    }
    return out
  }

  # The flag is on the object; every mutator above checks it first (decision 96).
  #: () -> self
  def freeze = %x{
    self.frozen.Store(true)
    return self
  }

  #: () -> bool
  def frozen? = %x{ Boolean(self.frozen.Load()) }

  #: (Integer) -> E?
  def at(i) = self[i]

  #: (Integer, *untyped) -> untyped
  def dig(i, *rest) = %x{ return rbDig(self, append([]any{i}, rest_...)) }

  # The first element that is an Array (or tuple) whose first item == key.
  #: (untyped) -> E?
  def assoc(key) = %x{ return rbAssoc(self, key, 0) }

  # The first element that is an Array (or tuple) whose second item == value.
  #: (untyped) -> E?
  def rassoc(value) = %x{ return rbAssoc(self, value, 1) }

  #: (*Integer) -> Array[E]
  def fetch_values(*idx) = idx.map { |i| fetch(i) }

  #: () { (E) -> bool } -> Integer?
  def bsearch_index = %x{
    i := sort.Search(len(self.s), func(i int) bool { return bool(blk(self.s[i])) })
    if i == len(self.s) {
      return nil
    }
    return Ref(Integer(i))
  }

  #: () { (E) -> bool } -> E?
  def rfind = %x{
    for i := len(self.s) - 1; i >= 0; i-- {
      if bool(blk(self.s[i])) {
        return &self.s[i]
      }
    }
    return nil
  }

  #: (Array[E]) -> self
  def replace(other) = %x{
    self.rbCheckFrozen()
    self.s = append([]E{}, other.s...)
    return self
  }
end
