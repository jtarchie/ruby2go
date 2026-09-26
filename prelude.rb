# prelude.rb
# rbs_inline: enabled
#
# The core library, compiled by the same transpiler as user code. It reopens
# core classes, so it never runs on MRI. Only `%x{}` leaves drop into Go.

# Top-level %x{} is emitted verbatim. Imports are resolved by goimports.
%x{
  var stdout = bufio.NewWriter(os.Stdout)

  // rbTopRecover turns an uncaught Ruby exception into exit status 1, like
  // MRI. Output is flushed first so partial output before a crash matches.
  func rbTopRecover() {
    if r := recover(); r != nil {
      stdout.Flush()
      if e, ok := r.(ExceptionI); ok {
        fmt.Fprintf(os.Stderr, "%s (%s)\\n", e.Message(), rbClassName(r))
      } else {
        fmt.Fprintf(os.Stderr, "%v\\n", r)
      }
      os.Exit(1)
    }
  }

  // Ref boxes a value into T? (represented as *T).
  func Ref[T any](v T) *T { return &v }

  // Opt converts T? to untyped: a nil *T must become an untyped nil or a
  // `case nil` type switch misses it.
  func Opt[T any](p *T) any {
    if p == nil {
      return nil
    }
    return *p
  }

  // OptOf converts untyped to T?.
  func OptOf[T any](a any) *T {
    if a == nil {
      return nil
    }
    v := a.(T)
    return &v
  }

  type I_ToS interface{ ToS() String }
  type I_Inspect interface{ Inspect() String }

  func rbToS(a any) String {
    if a == nil {
      return ""
    }
    return a.(I_ToS).ToS()
  }

  func rbInspect(a any) String {
    if a == nil {
      return "nil"
    }
    return a.(I_Inspect).Inspect()
  }

  func rbTruthy(a any) bool {
    switch v := a.(type) {
    case nil:
      return false
    case Boolean:
      return bool(v)
    }
    return true
  }

  func rbEq[T comparable](a, b T) Boolean {
    if e, ok := any(a).(interface{ Eq(any) Boolean }); ok {
      return e.Eq(any(b))
    }
    return Boolean(any(a) == any(b))
  }

  func rbCmp[T comparable](a, b T) Integer {
    return any(a).(interface{ Cmp(T) Integer }).Cmp(b)
  }

  func rbIdentical(a, b any) bool {
    switch a := a.(type) {
    case String:
      b, ok := b.(String)
      return ok && len(a) == len(b) && unsafe.StringData(string(a)) == unsafe.StringData(string(b))
    }
    return a == b
  }

  func rbIsA[I any](r any) bool {
    _, ok := r.(I)
    return ok
  }

  func rbClassName(a any) string {
    if a == nil {
      return "NilClass"
    }
    t := reflect.TypeOf(a)
    for t.Kind() == reflect.Ptr {
      t = t.Elem()
    }
    name := t.Name()
    if i := strings.IndexByte(name, '['); i >= 0 {
      name = name[:i]
    }
    return name
  }

  // rbWrapPanic converts Go runtime panics into Ruby exceptions so a
  // catch-all rescue sees a StandardError.
  func rbWrapPanic(r any) any {
    if _, ok := r.(ExceptionI); ok {
      return r
    }
    if err, ok := r.(runtime.Error); ok {
      msg := err.Error()
      if strings.Contains(msg, "divide by zero") {
        return NewZeroDivisionError(Ref[String]("divided by 0"))
      }
      if strings.Contains(msg, "nil pointer") {
        return NewNoMethodError(Ref[String]("undefined method for nil"))
      }
      if strings.Contains(msg, "index out of range") {
        return NewIndexError(Ref[String](String(msg)))
      }
      return NewStandardError(Ref[String](String(msg)))
    }
    return NewStandardError(Ref[String](String(fmt.Sprint(r))))
  }

  func NewHash[K, V comparable]() *Hash[K, V] { return &Hash[K, V]{vals: map[K]V{}} }

  // `when Array` / `when Hash` in a type switch can't match a generic
  // instantiation; every instantiation implements these instead.
  type Array_Any interface{ _ToAny() *Array[any] }
  type Hash_Any interface{ _ToAny() *Hash[any, any] }

  func rbFloatToS(f float64) String {
    switch {
    case math.IsNaN(f):
      return "NaN"
    case math.IsInf(f, 1):
      return "Infinity"
    case math.IsInf(f, -1):
      return "-Infinity"
    }
    abs := math.Abs(f)
    var s string
    if abs != 0 && (abs >= 1e16 || abs < 1e-4) {
      s = strconv.FormatFloat(f, 'e', -1, 64)
      mant, exp, _ := strings.Cut(s, "e")
      if !strings.Contains(mant, ".") {
        mant += ".0"
      }
      return String(mant + "e" + exp)
    }
    s = strconv.FormatFloat(f, 'f', -1, 64)
    if !strings.ContainsAny(s, ".") {
      s += ".0"
    }
    return String(s)
  }

  func rbStringInspect(s string) String {
    var b strings.Builder
    b.WriteByte('"')
    for _, r := range s {
      switch r {
      case '"':
        b.WriteString(`\\"`)
      case '\\\\':
        b.WriteString(`\\\\`)
      case '\\n':
        b.WriteString(`\\n`)
      case '\\t':
        b.WriteString(`\\t`)
      case '\\r':
        b.WriteString(`\\r`)
      case '\\f':
        b.WriteString(`\\f`)
      case '\\v':
        b.WriteString(`\\v`)
      case '\\a':
        b.WriteString(`\\a`)
      case '\\b':
        b.WriteString(`\\b`)
      case 0x1b:
        b.WriteString(`\\e`)
      case '#':
        b.WriteByte('#')
      default:
        if r < 0x20 || r == 0x7f {
          fmt.Fprintf(&b, `\\x%02X`, r)
        } else {
          b.WriteRune(r)
        }
      }
    }
    b.WriteByte('"')
    res := b.String()
    for _, c := range []string{string(rune(123)), "$", "@"} {
      res = strings.ReplaceAll(res, "#"+c, `\\#`+c)
    }
    return String(res)
  }
}

class BasicObject
  #: (untyped) -> bool
  def equal?(other) = %x{ Boolean(rbIdentical(self, other)) }

  #: (untyped) -> bool
  def ==(other) = %x{ Boolean(rbIdentical(self, other)) }

  #: (untyped) -> bool
  def !=(other) = !(self == other)

  #: () -> bool
  def ! = false
end

module Kernel
  # @rbs [X] () { (self) -> X } -> X
  def then = yield(self)

  #: () -> String
  def to_s = %x{ String("#<" + rbClassName(self) + ">") }

  #: () -> String
  def inspect = to_s

  #: () -> bool
  def nil? = false

  #: () -> bool
  def frozen? = true

  #: (?Integer) -> void
  def exit(status = 0) = %x{ stdout.Flush(); os.Exit(int(status)) }

  private

  #: (*untyped) -> nil
  def puts(*args)
    return __write("\n") if args.empty?

    args.each do |a|
      case a
      when nil then __write("\n")
      when Array then puts(*a)
      else
        s = a.to_s
        __write(s.end_with?("\n") ? s : s + "\n")
      end
    end
    nil
  end

  #: (*untyped) -> nil
  def print(*args)
    args.each { |a| __write(a.to_s) }
    nil
  end

  #: (String) -> nil
  def __write(s) = %x{ stdout.WriteString(string(s)) }

  #: () -> String
  def __class_name = %x{ String(rbClassName(self)) }
end

class Object < BasicObject
  include Kernel
end

module Comparable
  #: (self) -> Integer
  def <=>(other) = raise(NotImplementedError)

  #: (self) -> bool
  def <(other) = (self <=> other) < 0

  #: (self) -> bool
  def <=(other) = (self <=> other) <= 0

  #: (self) -> bool
  def >(other) = (self <=> other) > 0

  #: (self) -> bool
  def >=(other) = (self <=> other) >= 0

  #: (self, self) -> bool
  def between?(lo, hi) = !(self < lo) && !(hi < self)

  #: (self, self) -> self
  def clamp(lo, hi)
    return lo if self < lo
    return hi if (self <=> hi) > 0
    self
  end
end

# RBS `bool`; stands in for TrueClass/FalseClass.
# @go_type bool
class Boolean < Object
  #: () -> bool
  def ! = %x{ !self }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(Boolean)
    return Boolean(ok && self == o)
  }

  #: () -> String
  def to_s = %x{ String(strconv.FormatBool(bool(self))) }

  #: () -> String
  def inspect = to_s

  #: (bool) -> bool
  def &(other) = %x{ self && other }

  #: (bool) -> bool
  def |(other) = %x{ self || other }

  #: (bool) -> bool
  def ^(other) = %x{ self != other }
end

# @go_type int
class Integer < Object
  include Comparable

  #: (Integer) -> Integer
  def <=>(other) = %x{ Integer(cmp.Compare(self, other)) }

  #: (Integer) -> bool
  def <(other) = %x{ Boolean(self < other) }

  #: (Integer) -> bool
  def <=(other) = %x{ Boolean(self <= other) }

  #: (Integer) -> bool
  def >(other) = %x{ Boolean(self > other) }

  #: (Integer) -> bool
  def >=(other) = %x{ Boolean(self >= other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    switch o := other.(type) {
    case Integer:
      return Boolean(self == o)
    case Float:
      return Boolean(Float(self) == o)
    }
    return false
  }

  #: (Integer) -> Integer
  def +(other) = %x{ self + other }

  #: (Integer) -> Integer
  def -(other) = %x{ self - other }

  #: (Integer) -> Integer
  def *(other) = %x{ self * other }

  # Ruby floors; Go truncates.
  #: (Integer) -> Integer
  def /(other) = %x{
    if other == 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    q := self / other
    if self%other != 0 && (self < 0) != (other < 0) {
      q--
    }
    return q
  }

  #: (Integer) -> Integer
  def %(other) = %x{
    if other == 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    m := self % other
    if m != 0 && (m < 0) != (other < 0) {
      m += other
    }
    return m
  }

  #: (Integer) -> Integer
  def **(other) = %x{
    result := Integer(1)
    for i := Integer(0); i < other; i++ {
      result *= self
    }
    return result
  }

  #: () -> Integer
  def -@ = %x{ -self }

  #: () -> Integer
  def abs = %x{
    if self < 0 {
      return -self
    }
    return self
  }

  #: () -> bool
  def even? = %x{ self%2 == 0 }

  #: () -> bool
  def odd? = %x{ self%2 != 0 }

  #: () -> bool
  def zero? = %x{ self == 0 }

  #: () -> bool
  def positive? = %x{ self > 0 }

  #: () -> bool
  def negative? = %x{ self < 0 }

  #: () -> Integer
  def succ = self + 1

  #: () -> Integer
  def pred = self - 1

  #: () -> Integer
  def to_i = self

  #: () -> Float
  def to_f = %x{ Float(self) }

  #: () -> String
  def to_s = %x{ String(strconv.Itoa(int(self))) }

  #: () -> String
  def inspect = to_s

  #: () -> Integer
  def hash = self

  #: () -> String
  def chr = %x{ String(rune(self)) }

  #: () { (Integer) -> void } -> void
  def times = %x{
    return func(yield func(Integer) bool) {
      for i := Integer(0); i < self; i++ {
        if !yield(i) {
          return
        }
      }
    }
  }

  #: (Integer) { (Integer) -> void } -> void
  def upto(limit) = %x{
    return func(yield func(Integer) bool) {
      for i := self; i <= limit; i++ {
        if !yield(i) {
          return
        }
      }
    }
  }

  #: (Integer) { (Integer) -> void } -> void
  def downto(limit) = %x{
    return func(yield func(Integer) bool) {
      for i := self; i >= limit; i-- {
        if !yield(i) {
          return
        }
      }
    }
  }
end

# @go_type float64
class Float < Object
  include Comparable

  #: (Float) -> Integer
  def <=>(other) = %x{ Integer(cmp.Compare(self, other)) }

  #: (Float) -> bool
  def <(other) = %x{ Boolean(self < other) }

  #: (Float) -> bool
  def <=(other) = %x{ Boolean(self <= other) }

  #: (Float) -> bool
  def >(other) = %x{ Boolean(self > other) }

  #: (Float) -> bool
  def >=(other) = %x{ Boolean(self >= other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    switch o := other.(type) {
    case Float:
      return Boolean(self == o)
    case Integer:
      return Boolean(self == Float(o))
    }
    return false
  }

  #: (Float) -> Float
  def +(other) = %x{ self + other }

  #: (Float) -> Float
  def -(other) = %x{ self - other }

  #: (Float) -> Float
  def *(other) = %x{ self * other }

  #: (Float) -> Float
  def /(other) = %x{ self / other }

  #: (Float) -> Float
  def **(other) = %x{ Float(math.Pow(float64(self), float64(other))) }

  #: () -> Float
  def -@ = %x{ -self }

  #: () -> Float
  def abs = %x{ Float(math.Abs(float64(self))) }

  #: () -> Integer
  def to_i = %x{ Integer(self) }

  #: () -> Integer
  def floor = %x{ Integer(math.Floor(float64(self))) }

  #: () -> Integer
  def ceil = %x{ Integer(math.Ceil(float64(self))) }

  #: () -> Integer
  def round = %x{ Integer(math.RoundToEven(float64(self))) }

  #: () -> Float
  def to_f = self

  #: () -> bool
  def zero? = %x{ self == 0 }

  #: () -> bool
  def nan? = %x{ Boolean(math.IsNaN(float64(self))) }

  #: () -> String
  def to_s = %x{ rbFloatToS(float64(self)) }

  #: () -> String
  def inspect = to_s
end

# @go_type string
class String < Object
  include Comparable

  #: (String) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(String)
    return Boolean(ok && self == o)
  }

  #: (String) -> String
  def +(other) = %x{ self + other }

  #: (Integer) -> String
  def *(n) = %x{ String(strings.Repeat(string(self), int(n))) }

  #: () -> String
  def to_s = self

  #: () -> String
  def to_str = self

  #: () -> String
  def inspect = %x{ rbStringInspect(string(self)) }

  #: () -> String
  def dup = %x{ String(strings.Clone(string(self))) }

  #: () -> String
  def upcase = %x{ String(strings.ToUpper(string(self))) }

  #: () -> String
  def downcase = %x{ String(strings.ToLower(string(self))) }

  #: () -> String
  def capitalize = %x{
    s := strings.ToLower(string(self))
    if s == "" {
      return ""
    }
    r, n := utf8.DecodeRuneInString(s)
    return String(string(unicode.ToUpper(r)) + s[n:])
  }

  #: () -> String
  def reverse = %x{
    r := []rune(string(self))
    for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
      r[i], r[j] = r[j], r[i]
    }
    return String(r)
  }

  #: () -> String
  def strip = %x{ String(strings.TrimSpace(string(self))) }

  #: () -> String
  def lstrip = %x{ String(strings.TrimLeft(string(self), " \\t\\n\\r\\f\\v")) }

  #: () -> String
  def rstrip = %x{ String(strings.TrimRight(string(self), " \\t\\n\\r\\f\\v")) }

  #: () -> String
  def chomp = %x{ String(strings.TrimSuffix(strings.TrimSuffix(string(self), "\\n"), "\\r")) }

  #: () -> Integer
  def size = %x{ Integer(utf8.RuneCountInString(string(self))) }

  #: () -> Integer
  def length = size

  #: () -> Integer
  def bytesize = %x{ Integer(len(self)) }

  #: () -> bool
  def empty? = %x{ self == "" }

  #: (String) -> bool
  def end_with?(s) = %x{ Boolean(strings.HasSuffix(string(self), string(s))) }

  #: (String) -> bool
  def start_with?(s) = %x{ Boolean(strings.HasPrefix(string(self), string(s))) }

  #: (String) -> bool
  def include?(s) = %x{ Boolean(strings.Contains(string(self), string(s))) }

  #: (String) -> Integer?
  def index(s) = %x{
    i := strings.Index(string(self), string(s))
    if i < 0 {
      return nil
    }
    return Ref(Integer(utf8.RuneCountInString(string(self)[:i])))
  }

  #: (Integer) -> String?
  def [](i) = %x{
    r := []rune(string(self))
    if i < 0 {
      i += Integer(len(r))
    }
    if i < 0 || int(i) >= len(r) {
      return nil
    }
    return Ref(String(r[i]))
  }

  #: () -> Array[String]
  def chars = %x{
    out := &Array[String]{}
    for _, r := range string(self) {
      *out = append(*out, String(r))
    }
    return out
  }

  #: () { (String) -> void } -> void
  def each_char = %x{
    return func(yield func(String) bool) {
      for _, r := range string(self) {
        if !yield(String(r)) {
          return
        }
      }
    }
  }

  #: () -> Array[String]
  def lines = %x{
    out := &Array[String]{}
    for _, l := range strings.SplitAfter(string(self), "\\n") {
      if l != "" {
        *out = append(*out, String(l))
      }
    }
    return out
  }

  # Ruby's no-argument split: on whitespace, dropping empties.
  #: () -> Array[String]
  def split = %x{
    out := &Array[String]{}
    for _, f := range strings.Fields(string(self)) {
      *out = append(*out, String(f))
    }
    return out
  }

  #: (String) -> Array[String]
  def split_on(sep) = %x{
    parts := strings.Split(string(self), string(sep))
    for len(parts) > 0 && parts[len(parts)-1] == "" {
      parts = parts[:len(parts)-1]
    }
    out := &Array[String]{}
    for _, p := range parts {
      *out = append(*out, String(p))
    }
    return out
  }

  #: (String, String) -> String
  def sub(from, to) = %x{ String(strings.Replace(string(self), string(from), string(to), 1)) }

  #: (String, String) -> String
  def gsub(from, to) = %x{ String(strings.ReplaceAll(string(self), string(from), string(to))) }

  #: (String, String) -> String
  def tr(from, to) = %x{
    f, t := []rune(string(from)), []rune(string(to))
    return String(strings.Map(func(r rune) rune {
      for i, c := range f {
        if c == r {
          if i < len(t) {
            return t[i]
          }
          return t[len(t)-1]
        }
      }
      return r
    }, string(self)))
  }

  #: () -> Integer
  def to_i = %x{
    s := strings.TrimSpace(string(self))
    end := 0
    if end < len(s) && (s[end] == '-' || s[end] == '+') {
      end++
    }
    for end < len(s) && s[end] >= '0' && s[end] <= '9' {
      end++
    }
    n, _ := strconv.Atoi(s[:end])
    return Integer(n)
  }

  #: () -> Float
  def to_f = %x{
    f, _ := strconv.ParseFloat(strings.TrimSpace(string(self)), 64)
    return Float(f)
  }

  #: () -> Integer
  def ord = %x{
    r, _ := utf8.DecodeRuneInString(string(self))
    return Integer(r)
  }

  #: () -> Integer
  def hash = %x{
    h := fnv.New64a()
    h.Write([]byte(self))
    return Integer(h.Sum64())
  }

  #: () -> String
  def freeze = self

  #: (Integer) -> String
  def center(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return self
    }
    return String(strings.Repeat(" ", n/2) + string(self) + strings.Repeat(" ", n-n/2))
  }

  #: (Integer) -> String
  def ljust(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return self
    }
    return self + String(strings.Repeat(" ", n))
  }

  #: (Integer) -> String
  def rjust(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return self
    }
    return String(strings.Repeat(" ", n)) + self
  }
end

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
  def each_entry = %x{ return self.Each() }

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

  #: (String) -> String
  def join(sep) = %x{
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
    out := &Array[any]{}
    for _, x := range *self {
      *out = append(*out, x)
    }
    return out
  }
end

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

  #: (K) -> V
  def fetch(k)
    v = self[k]
    return v if v
    raise KeyError, "key not found: #{k.inspect}"
  end

  #: (K, V) -> V
  def fetch_or(k, default)
    v = self[k]
    return v if v
    default
  end

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
    out := NewHash[any, any]()
    for _, k := range self.keys {
      out.IdxSet(k, self.vals[k])
    }
    return out
  }
end

class Exception < Object
  #: (?String?) -> void
  def initialize(message = nil)
    @message = message
  end

  #: () -> String
  def message
    m = @message
    return m if m
    __class_name
  end

  #: () -> String
  def to_s = message

  #: () -> String
  def inspect = "#<#{__class_name}: #{message}>"

  #: () -> String?
  def backtrace = nil
end

class StandardError < Exception; end
class RuntimeError < StandardError; end
class ArgumentError < StandardError; end
class TypeError < StandardError; end
class NameError < StandardError; end
class NoMethodError < NameError; end
class IndexError < StandardError; end
class KeyError < IndexError; end
class StopIteration < IndexError; end
class RangeError < StandardError; end
class ZeroDivisionError < StandardError; end
class ScriptError < Exception; end
class NotImplementedError < ScriptError; end
