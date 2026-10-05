# prelude/array.rb
# rbs_inline: enabled
#
# Array as a named Go slice, always handled as a pointer.

# Array is mutable and aliased in Ruby, so it is always handled as a pointer.
# @rbs generic E
# @go_type []E
class Array < Object
  include Enumerable #[E]

  # Every slot holds the same v, as MRI's (mutating a shared Array element shows in all).
  #: [X] (Integer, X) -> Array[X]
  def self.new(n, v) = %x{
    if n < 0 {
      panic(NewArgumentError(Ref(String("negative array size"))))
    }
    out := make(Array[X], n)
    for i := range out {
      out[i] = v
    }
    return &out
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
        if i >= len(*self) || !yield((*self)[i]) {
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
    rbFrozenCheck(self)
    if i < 0 {
      if -i > Integer(len(*self)) {
        panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", i, len(*self))))))
      }
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
    rbFrozenCheck(self)
    *self = append(*self, x)
    return self
  }

  #: (E) -> self
  def push(x) = self << x

  #: (E) -> self
  def append(x) = self << x

  #: () -> E?
  def pop = %x{
    rbFrozenCheck(self)
    if len(*self) == 0 {
      return nil
    }
    x := (*self)[len(*self)-1]
    *self = (*self)[:len(*self)-1]
    return &x
  }

  #: () -> E?
  def shift = %x{
    rbFrozenCheck(self)
    if len(*self) == 0 {
      return nil
    }
    x := (*self)[0]
    *self = (*self)[1:]
    return &x
  }

  #: (E) -> self
  def unshift(x) = %x{
    rbFrozenCheck(self)
    *self = append([]E{x}, *self...)
    return self
  }

  #: (Array[E]) -> self
  def concat(other) = %x{
    rbFrozenCheck(self)
    *self = append(*self, *other...)
    return self
  }

  #: (Array[E]) -> Array[E]
  def +(other) = %x{
    out := &Array[E]{}
    *out = append(append(*out, *self...), *other...)
    return out
  }

  # Set operations match elements by eql?/hash, as MRI's.
  #: (Array[E]) -> Array[E]
  def -(other) = %x{
    drop := rbNewKeySet(*other)
    out := &Array[E]{}
    for _, x := range *self {
      if !drop.has(x) {
        *out = append(*out, x)
      }
    }
    return out
  }

  #: (Array[E]) -> Array[E]
  def difference(other) = self - other

  # Only C, c and U so far (decision 136); any other directive raises NotImplementedError rather than packing wrong bytes.
  #: (String) -> String
  def pack(format) = %x{
    items := make([]any, len(*self))
    for i, x := range *self {
      items[i] = x
    }
    return String(rbPack(items, string(format)))
  }

  #: (Array[E]) -> Array[E]
  def &(other) = %x{
    keep := rbNewKeySet(*other)
    seen := rbNewKeySet[E](nil)
    out := &Array[E]{}
    for _, x := range *self {
      if keep.has(x) && !seen.has(x) {
        seen.put(x)
        *out = append(*out, x)
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
    i := sort.Search(len(*self), func(i int) bool { return bool(blk((*self)[i])) })
    if i == len(*self) {
      return nil
    }
    return &(*self)[i]
  }

  #: () -> self
  def sort! = %x{
    rbFrozenCheck(self)
    slices.SortStableFunc(*self, func(a, b E) int { return -int(rbCmp(b, a)) }) // (earlier, later), as MRI's failure names them
    return self
  }

  #: [K] () { (E) -> K } -> self
  def sort_by! = %x{
    rbFrozenCheck(self)
    keys := make(map[int]K, len(*self))
    idx := make([]int, len(*self))
    for i, x := range *self {
      idx[i], keys[i] = i, blk(x)
    }
    slices.SortStableFunc(idx, func(a, b int) int { return -int(rbCmp(keys[b], keys[a])) })
    out := make(Array[E], len(*self))
    for i, j := range idx {
      out[i] = (*self)[j]
    }
    *self = out
    return self
  }

  #: () { (E) -> E } -> self
  def map! = %x{
    rbFrozenCheck(self)
    for i, x := range *self {
      (*self)[i] = blk(x)
    }
    return self
  }

  #: () { (E) -> E } -> self
  def collect!(&block) = map!(&block)

  #: () { (E) -> bool } -> self
  def keep_if = %x{
    rbFrozenCheck(self)
    out := (*self)[:0]
    for _, x := range *self {
      if bool(blk(x)) {
        out = append(out, x)
      }
    }
    clear((*self)[len(out):])
    *self = out
    return self
  }

  #: () { (E) -> bool } -> self
  def delete_if = %x{
    rbFrozenCheck(self)
    out := (*self)[:0]
    for _, x := range *self {
      if !bool(blk(x)) {
        out = append(out, x)
      }
    }
    clear((*self)[len(out):])
    *self = out
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
    rbFrozenCheck(self)
    u := Array_Uniq(self)
    if len(*u) == len(*self) {
      return nil
    }
    *self = *u
    return &self
  }

  #: () -> self
  def reverse! = %x{
    rbFrozenCheck(self)
    slices.Reverse(*self)
    return self
  }

  #: (Integer, *E) -> self
  def insert(i, *objs) = %x{
    rbFrozenCheck(self)
    n := int(i)
    if n < 0 {
      n += len(*self) + 1
    }
    if n < 0 {
      panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", int(i), len(*self)+1)))))
    }
    for len(*self) < n {
      var zero E
      *self = append(*self, zero)
    }
    *self = slices.Insert(*self, n, rest_...)
    return self
  }

  #: (E) -> self
  def fill(v) = %x{
    rbFrozenCheck(self)
    for i := range *self {
      (*self)[i] = v
    }
    return self
  }

  #: () { (Integer) -> void } -> void
  def each_index = %x{
    return func(yield func(Integer) bool) {
      for i := 0; ; i++ { // not range len: the block may grow the array
        if i >= len(*self) || !yield(Integer(i)) {
          return
        }
      }
    }
  }

  #: () -> Array[Integer]
  def __each_index_enum = %x{
    out := &Array[Integer]{}
    for i := range len(*self) {
      *out = append(*out, Integer(i))
    }
    return out
  }

  #: () -> Array[E]
  def __each_enum = self

  # `each.with_index(1) { |x, i| … }`: blockless each is the Array itself.
  #: (?Integer) { (E, Integer) -> void } -> void
  def with_index(offset = 0) = %x{
    return func(yield func(E, Integer) bool) {
      for i := 0; ; i++ { // not range len: the block may grow the array
        if i >= len(*self) || !yield((*self)[i], Integer(i)+offset) {
          return
        }
      }
    }
  }

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
    rbFrozenCheck(self)
    *self = (*self)[:0]
    return self
  }

  #: (E) -> E?
  def delete(v) = %x{
    rbFrozenCheck(self)
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
    rbFrozenCheck(self)
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

  # flatten on elements that are not Arrays: a copy, or, for untyped
  # elements, MRI's dynamic splice. Typed nesting goes to the @self forms
  # below (decision 92).
  #: (?Integer) -> Array[E]
  def flatten(depth = -1) = %x{
    if a, ok := any(self).(*Array[any]); ok {
      out := &Array[any]{}
      rbFlattenInto(out, *a, int(depth))
      return any(out).(*Array[E])
    }
    out := slices.Clone(*self)
    return &out
  }

  # @self Array[Array[Array[U]]]
  # @rbs [U] () -> Array[U]
  def __flatten_nested3 = %x{
    rows := make([]*Array[U], 0, len(*self))
    for _, r := range *self {
      rows = append(rows, *r...)
    }
    return rbFlattenRows(rows, -1)
  }

  # @self Array[Array[U]]
  # @rbs [U] (?Integer) -> Array[U]
  def __flatten_nested(depth = -1) = %x{
    if depth == 0 { // the result would be Array[Array[U]], not this signature's Array[U]
      panic(NewNotImplementedError(Ref(String("rb2go: flatten(0) of nested Arrays; use dup"))))
    }
    return rbFlattenRows(*self, int(depth))
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
    if len(*self) == 0 {
      return out
    }
    n := len(*(*self)[0])
    for _, r := range *self {
      if len(*r) != n {
        panic(NewIndexError(Ref(String(fmt.Sprintf("element size differs (%d should be %d)", len(*r), n)))))
      }
    }
    for j := range n {
      col := make(Array[U], len(*self))
      for i, r := range *self {
        col[i] = (*r)[j]
      }
      *out = append(*out, &col)
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
    i := sort.Search(len(*self), func(i int) bool { return bool(blk((*self)[i])) })
    if i == len(*self) {
      return nil
    }
    return Ref(Integer(i))
  }

  #: () { (E) -> bool } -> E?
  def rfind = %x{
    for i := len(*self) - 1; i >= 0; i-- {
      if bool(blk((*self)[i])) {
        return &(*self)[i]
      }
    }
    return nil
  }

  #: (Array[E]) -> self
  def replace(other) = %x{
    rbFrozenCheck(self)
    *self = append(Array[E]{}, *other...)
    return self
  }
end
