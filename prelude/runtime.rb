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
    if s, ok := a.(I_ToS); ok {
      return s.ToS()
    }
    return rbObjToS(a)
  }

  // rbObjToS is Kernel#to_s; only heap objects have an address to show.
  func rbObjToS(a any) String {
    s := "#<" + rbClassName(a)
    if v := reflect.ValueOf(a); v.Kind() == reflect.Pointer {
      s += fmt.Sprintf(":0x%016x", v.Pointer())
    }
    return String(s + ">")
  }

  // rbIvar feeds Kernel#inspect; !opt means nil was never assigned, which MRI doesn't list.
  type rbIvar struct {
    name string
    val  any
    opt  bool
  }

  var (
    rbInspectMu   sync.Mutex
    rbInspectBusy = map[any]bool{} // objects whose inspect is running, for MRI's "..."
  )

  // ponytail: the busy set is shared by threads, so concurrent inspects of one object print "..."; key it per goroutine if that shows up.
  func rbObjInspect(a any) String {
    o, ok := a.(interface{ _Ivars() []rbIvar })
    if !ok {
      return rbObjToS(a)
    }
    s := string(rbObjToS(a))
    var b strings.Builder
    b.WriteString(s[:len(s)-1])
    rbInspectMu.Lock()
    busy := rbInspectBusy[a]
    rbInspectBusy[a] = true
    rbInspectMu.Unlock()
    if busy {
      return String(b.String() + " ...>")
    }
    defer func() {
      rbInspectMu.Lock()
      delete(rbInspectBusy, a)
      rbInspectMu.Unlock()
    }()
    sep := " "
    for _, iv := range o._Ivars() {
      if !iv.opt && (iv.val == nil || rbNilPtr(iv.val)) {
        continue
      }
      b.WriteString(sep + iv.name + "=")
      sep = ", "
      if _, ok := iv.val.(I_Inspect); ok || iv.val == nil {
        b.WriteString(string(rbInspect(iv.val)))
      } else { // a Go value behind a prelude ivar
        b.WriteString(string(rbObjToS(iv.val)))
      }
    }
    return String(b.String() + ">")
  }

  func rbNilPtr(a any) bool {
    v := reflect.ValueOf(a)
    return v.Kind() == reflect.Pointer && v.IsNil()
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
    if e, ok := any(a).(interface{ _EqAny(any) Boolean }); ok {
      return e._EqAny(any(b)) // a == typed on its argument; see emitEqAdapter
    }
    return Boolean(any(a) == any(b))
  }

  // rbCmp is <=> for sort, min and max. Typed values have Cmp(T); untyped
  // ones (T is any) go through the generated DynCmp wrappers, which answer
  // nil for an incomparable argument, as MRI's <=> does.
  func rbCmp[T comparable](a, b T) Integer {
    if c, ok := any(a).(interface{ Cmp(T) Integer }); ok {
      return c.Cmp(b)
    }
    if c, ok := any(a).(interface{ DynCmp(...any) any }); ok {
      if r, ok := c.DynCmp(b).(Integer); ok {
        return r
      }
    }
    panic(rbCmpErr(a, b))
  }

  // rbCmpErr is MRI's rb_cmperr, raised where <=> answered nil: it
  // inspects immediates and Floats and names the class of anything else.
  func rbCmpErr(a, b any) *ArgumentError {
    with := rbClassName(b)
    switch b.(type) {
    case nil, Boolean, Integer, Float, Symbol:
      with = string(rbInspect(b))
    }
    return NewArgumentError(Ref(String("comparison of " + rbClassName(a) + " with " + with + " failed")))
  }

  // rbHash is #hash on a value of unknown type.
  func rbHash(a any) Integer {
    if h, ok := a.(interface{ Hash() Integer }); ok {
      return h.Hash()
    }
    // ponytail: no #hash (Float, Array, plain objects) hashes the inspect
    // text; an object's carries its address but also its ivars, so hash the
    // address alone once objects have an object id.
    h := fnv.New64a()
    _, _ = h.Write([]byte(rbInspect(a)))
    return Integer(h.Sum64())
  }

  func rbIdentical(a, b any) bool {
    if as, ok := a.(String); ok {
      bs, ok := b.(String)
      // Identity of immutable strings is their backing pointer.
      return ok && len(as) == len(bs) && unsafe.StringData(string(as)) == unsafe.StringData(string(bs)) //nolint:gosec // pointer compare only
    }
    return a == b
  }

  // rbStrID is a String's identity: backing pointer and length, as in rbIdentical.
  type rbStrID struct {
    p *byte
    n int
  }

  var (
    rbFrozenMu   sync.Mutex
    rbFrozenStrs map[rbStrID]bool // the literals (seeded on first use) and every String#freeze receiver
  )

  // rbStrFrozen reports whether s is a literal or was frozen, and with
  // freeze also marks it. Strings built at run time have fresh backing
  // arrays, so they aren't in the set, like MRI's unfrozen strings.
  // ponytail: frozen computed strings stay reachable from the set; use weak pointers if that leak shows up.
  func rbStrFrozen(s string, freeze bool) bool {
    id := rbStrID{unsafe.StringData(s), len(s)} //nolint:gosec // identity only
    rbFrozenMu.Lock()
    defer rbFrozenMu.Unlock()
    if rbFrozenStrs == nil {
      rbFrozenStrs = make(map[rbStrID]bool, len(rbStringLits))
      for _, l := range rbStringLits {
        rbFrozenStrs[rbStrID{unsafe.StringData(l), len(l)}] = true //nolint:gosec // identity only
      }
    }
    was := rbFrozenStrs[id]
    if freeze {
      rbFrozenStrs[id] = true
    }
    return was
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

  // rbFloatToI converts like MRI: a non-finite Float raises FloatDomainError.
  func rbFloatToI(f float64) Integer {
    if math.IsNaN(f) || math.IsInf(f, 0) {
      panic(NewFloatDomainError(Ref(rbFloatToS(f))))
    }
    if f < -(1<<63) || f >= 1<<63 {
      panic(NewRangeError(Ref("float " + rbFloatToS(f) + " out of range of integer")))
    }
    return Integer(f)
  }

  // rbIntOverflow raises where MRI would promote to a Bignum: Integer is a
  // Go int (decision 35). Out of line so the checked operators still inline.
  //go:noinline
  func rbIntOverflow(a Integer, op string, b Integer) {
    panic(NewRangeError(Ref(String(fmt.Sprintf("%d %s %d overflows Integer (64-bit; no Bignum)", a, op, b)))))
  }

  // rbIntMul is Integer#* past its 32-bit fast path, out of line too.
  //go:noinline
  func rbIntMul(a, b Integer) Integer {
    r, ok := rbIntMulOk(a, b)
    if !ok {
      rbIntOverflow(a, "*", b)
    }
    return r
  }

  // rbIntMulOk is a*b and whether it fits: the signed high word of the
  // 128-bit product must be the low word's sign extension.
  func rbIntMulOk(a, b Integer) (Integer, bool) {
    hi, lo := bits.Mul64(uint64(a), uint64(b))
    if a < 0 {
      hi -= uint64(b)
    }
    if b < 0 {
      hi -= uint64(a)
    }
    return Integer(lo), int64(hi) == int64(lo)>>63
  }

  func rbFloatToS(f float64) String {
    switch {
    case math.IsNaN(f):
      return "NaN"
    case math.IsInf(f, 1):
      return "Infinity"
    case math.IsInf(f, -1):
      return "-Infinity"
    }
    // MRI's flo_to_s: exponent form when the decimal point sits more than
    // DBL_DIG (15) digits in and the shortest digits have no fraction. In
    // [1e15, 1e16) the shortest digits lack a fraction iff f is integral.
    abs := math.Abs(f)
    var s string
    if abs != 0 && (abs >= 1e16 || abs < 1e-4 || abs >= 1e15 && f == math.Trunc(f)) {
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
    for i := 0; i < len(s); {
      r, n := utf8.DecodeRuneInString(s[i:])
      if r == utf8.RuneError && n == 1 {
        fmt.Fprintf(&b, `\\x%02X`, s[i])
        i++
        continue
      }
      i += n
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
        switch {
        case r < 0x20 || r == 0x7f:
          // ponytail: strings carry no encoding, so ASCII controls get the
          // US-ASCII form (Integer#chr, ASCII-only symbols); a UTF-8 literal
          // is \\u0000 in MRI. Needs an encoding bit on String.
          fmt.Fprintf(&b, `\\x%02X`, r)
        case unicode.IsGraphic(r) || unicode.In(r, unicode.Cf, unicode.Co):
          // MRI's "printable" for UTF-8: graphic, format or private use.
          b.WriteRune(r)
        case r > 0xFFFF:
          fmt.Fprintf(&b, `\\u{%X}`, r)
        default:
          fmt.Fprintf(&b, `\\u%04X`, r)
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
