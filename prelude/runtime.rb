# prelude/runtime.rb
# rbs_inline: enabled
#
# Go helpers that are not methods: output buffering, boxing for T?,
# dispatch on untyped values, exception plumbing.

%x{
  var stdout = bufio.NewWriter(os.Stdout)

  // stdoutMu serializes writes: threads are goroutines, and there is no GVL.
  var stdoutMu sync.Mutex

  func rbWrite(s string) {
    stdoutMu.Lock()
    defer stdoutMu.Unlock()
    _, _ = stdout.WriteString(s)
  }

  // rbTopRecover turns an uncaught Ruby exception into exit status 1, like
  // MRI. Output is flushed first so partial output before a crash matches.
  func rbTopRecover() {
    if r := recover(); r != nil {
      _ = stdout.Flush()
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

  // rbZero fills a left-out argument; the callee sees rbArgc and runs its own default.
  func rbZero[T any]() (z T) { return z }

  // Opt converts T? to untyped: a nil *T must become an untyped nil or a
  // `case nil` type switch misses it.
  func Opt[T any](p *T) any {
    if p == nil {
      return nil
    }
    return *p
  }

  // rbFlat collapses a generic E? instantiated with E = T? (a **T) to T?:
  // Ruby has one nil.
  func rbFlat[T any](p **T) *T {
    if p == nil {
      return nil
    }
    return *p
  }

  // OptOf converts untyped to T?; want names T for rbAs's TypeError.
  func OptOf[T any](a any, want string) *T {
    if a == nil {
      return nil
    }
    v := rbAs[T](a, want)
    return &v
  }

  // rbAs converts untyped to T, raising MRI's TypeError when a is not one:
  // a bare a.(T) would panic as a Go error, a StandardError.
  func rbAs[T any](a any, want string) T {
    v, ok := a.(T)
    if !ok {
      return rbAsSlow[T](a, want)
    }
    return v
  }

  // rbAsSlow converts as rbConv does (nil for untyped, other Array/Hash
  // instantiations, an Array into a tuple), else raises.
  func rbAsSlow[T any](a any, want string) T {
    if v, ok := rbConv[T](a); ok {
      return v
    }
    panic(rbConvError(a, want))
  }

  // rbConvError is MRI's "no implicit conversion" TypeError.
  func rbConvError(a any, want string) any {
    name := rbClassName(a)
    switch r := a.(type) {
    case nil:
      name = "nil"
    case Boolean:
      name = strconv.FormatBool(bool(r))
    case rbModule:
      name = strings.ToUpper(r._Kind()[:1]) + r._Kind()[1:]
    }
    return NewTypeError(Ref(String("no implicit conversion of " + name + " into " + want)))
  }

  type I_ToS interface{ ToS() String }
  type I_Inspect interface{ Inspect() String }

  // rbToS, rbInspect, rbEq and rbCmp take E = T? values from generic
  // code as the box itself; rbUnbox (generated) opens it.
  func rbToS(a any) String {
    a = rbUnbox(a)
    if a == nil {
      return ""
    }
    if s, ok := a.(I_ToS); ok {
      return s.ToS()
    }
    return String("#<" + rbClassName(a) + ">")
  }

  func rbInspect(a any) String {
    a = rbUnbox(a)
    if a == nil {
      return "nil"
    }
    return a.(I_Inspect).Inspect()
  }

  // rbInspecting holds the containers whose inspect is on the stack, so one
  // that holds itself prints [...] / {...} like MRI instead of overflowing.
  // ponytail: one set for all threads (MRI's is per-thread), so two threads
  // inspecting the same container at once may see [...]; a goroutine-local
  // set needs a goroutine id Go does not expose.
  var (
    rbInspectingMu sync.Mutex
    rbInspecting   = map[any]struct{}{}
  )

  // rbInspectEnter marks p as being inspected; false means it already is.
  // A true result must be paired with a deferred rbInspectLeave(p).
  func rbInspectEnter(p any) bool {
    rbInspectingMu.Lock()
    defer rbInspectingMu.Unlock()
    if _, ok := rbInspecting[p]; ok {
      return false
    }
    rbInspecting[p] = struct{}{}
    return true
  }

  func rbInspectLeave(p any) {
    rbInspectingMu.Lock()
    defer rbInspectingMu.Unlock()
    delete(rbInspecting, p)
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

  // rbTruthyOpt tests a Boolean?: false is as falsy as nil.
  func rbTruthyOpt(p *Boolean) bool { return p != nil && bool(*p) }

  func rbEq[T comparable](a, b T) Boolean {
    x, y := rbUnbox(any(a)), rbUnbox(any(b))
    if e, ok := x.(interface{ Op_eq(any) Boolean }); ok {
      return e.Op_eq(y)
    }
    return Boolean(x == y)
  }

  // Hash keys, uniq and tally match by Ruby's eql?/hash. For most keys that
  // is Go ==: strings, numbers, symbols, and objects by identity. Arrays,
  // hashes, Regexps, Structs and anything else defining both eql? and hash
  // match by value, and so does a T? box (*T) by what it points at.

  // rbPlainKey reports whether K's Go == is eql?, so a map can key by K.
  func rbPlainKey[K comparable]() bool {
    var z K
    switch z := any(z).(type) {
    case String, Symbol, Integer, Float, Boolean:
      return true
    case interface{ rbPlain() bool }:
      return z.rbPlain()
    }
    return false
  }

  type rbEqlHash interface {
    EqlQ(other any) Boolean
    Hash() Integer
  }

  var rbHashSeed = maphash.MakeSeed()

  // rbValueKey is k's hash when k matches by value rather than Go ==.
  func rbValueKey(k any) (uint64, bool) {
    if p, ok := rbKeyUnbox(k); ok {
      return rbKeyHash(p), true
    }
    if v, ok := k.(rbEqlHash); ok {
      return uint64(v.Hash()), true
    }
    return 0, false
  }

  func rbKeyHash(k any) uint64 {
    if h, ok := rbValueKey(k); ok {
      return h
    }
    return maphash.Comparable(rbHashSeed, k)
  }

  func rbKeyEql(a, b any) bool {
    if p, ok := rbKeyUnbox(a); ok {
      a = p
    }
    if p, ok := rbKeyUnbox(b); ok {
      b = p
    }
    if a == b {
      return true
    }
    if e, ok := a.(rbEqlHash); ok {
      return bool(e.EqlQ(b))
    }
    return false
  }

  // rbHash is #hash on an untyped value.
  func rbHash(a any) Integer {
    if h, ok := a.(interface{ Hash() Integer }); ok {
      return h.Hash()
    }
    return Integer(rbKeyHash(a))
  }

  // rbKeyUnbox returns what a T? box holds, for any box (the generated
  // rbUnbox knows only the T? types the program renders; a prelude generic
  // can make others). Boxes point at values (named basic types, tuples,
  // interfaces, pointers); pointers to other structs, and to slices and
  // maps, are objects.
  func rbKeyUnbox(k any) (any, bool) {
    t := reflect.TypeOf(k)
    if t == nil || t.Kind() != reflect.Pointer {
      return nil, false
    }
    e := t.Elem()
    if kind := e.Kind(); kind == reflect.Slice || kind == reflect.Map || kind == reflect.Struct && !e.Implements(rbPlainType) {
      return nil, false
    }
    v := reflect.ValueOf(k)
    if v.IsNil() {
      return nil, true
    }
    return v.Elem().Interface(), true
  }

  var rbPlainType = reflect.TypeFor[interface{ rbPlain() bool }]()

  func rbCmp[T comparable](a, b T) Integer {
    if c, ok := any(a).(interface{ Op_cmp(T) Integer }); ok {
      return c.Op_cmp(b)
    }
    return rbCmpBox(any(a), any(b))
  }

  // rbCmpOpt compares the values in two T? boxes (see rbCmpBox).
  func rbCmpOpt[T comparable](a, b *T) Integer {
    if a == nil || b == nil {
      return rbCmpFailed(a, b)
    }
    return rbCmp(*a, *b)
  }

  // rbCmpFailed is MRI's rb_cmperr.
  func rbCmpFailed(a, b any) Integer {
    a, b = rbUnbox(a), rbUnbox(b)
    desc := rbClassName(b)
    switch b.(type) {
    case nil, Boolean, Integer, Float:
      desc = string(rbInspect(b))
    }
    panic(NewArgumentError(Ref(String("comparison of " + rbClassName(a) + " with " + desc + " failed"))))
  }

  func rbIdentical(a, b any) bool {
    if as, ok := a.(String); ok {
      bs, ok := b.(String)
      // Identity of immutable strings is their backing pointer.
      return ok && len(as) == len(bs) && unsafe.StringData(string(as)) == unsafe.StringData(string(bs)) //nolint:gosec // pointer compare only
    }
    return a == b
  }

  func rbIsA[I any](r any) bool {
    _, ok := r.(I)
    return ok
  }

  // rbSplat converts a splatted array's elements to a rest param's type.
  func rbSplat[T, E any](s []T, conv func(T) E) []E {
    out := make([]E, len(s))
    for i, v := range s {
      out[i] = conv(v)
    }
    return out
  }

  func rbClassName(a any) string {
    if a == nil {
      return "NilClass"
    }
    t := reflect.TypeOf(a)
    for t.Kind() == reflect.Pointer {
      t = t.Elem()
    }
    name := t.Name()
    if i := strings.IndexByte(name, '['); i >= 0 {
      name = name[:i]
    }
    if ruby, ok := rbRubyNames[name]; ok {
      return ruby
    }
    return name
  }

  // rbWrapPanic converts Go runtime panics into Ruby exceptions so a
  // catch-all rescue sees a StandardError.
  func rbWrapPanic(r any) any {
    switch r.(type) {
    case ExceptionI, rbStop:
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

  // rbStop unwinds a closure-taking each when the loop over its rbSeq
  // adapter stops early, like MRI's break: ensure runs, rescue passes it on.
  type rbStop struct{}

  // rbSeq adapts a closure-taking each to the iter.Seq its callers range
  // over (decision 4). A loop body's own exception still unwinds through
  // each; if each rescues it, Go aborts, since a range function may not
  // recover one.
  func rbSeq[E any](each func(func(E))) iter.Seq[E] {
    return func(yield func(E) bool) {
      defer rbStopped()
      stopped := false
      each(func(x E) {
        if stopped || !yield(x) {
          stopped = true
          panic(rbStop{})
        }
      })
    }
  }

  func rbSeq2[K, V any](each func(func(K, V))) iter.Seq2[K, V] {
    return func(yield func(K, V) bool) {
      defer rbStopped()
      stopped := false
      each(func(k K, v V) {
        if stopped || !yield(k, v) {
          stopped = true
          panic(rbStop{})
        }
      })
    }
  }

  func rbStopped() {
    if r := recover(); r != nil {
      if _, ok := r.(rbStop); !ok {
        panic(r)
      }
    }
  }

  func NewHash[K, V comparable]() *Hash[K, V] {
    return &Hash[K, V]{vals: map[K]V{}, idx: rbKeyIndex[K]{plain: rbPlainKey[K]()}}
  }

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
