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
    return String("#<" + rbClassName(a) + ">")
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
    // MRI inspects immediates and names the class of anything else.
    with := rbClassName(b)
    switch any(b).(type) {
    case nil, Boolean, Integer, Float, Symbol:
      with = string(rbInspect(b))
    }
    panic(NewArgumentError(Ref(String("comparison of " + rbClassName(a) + " with " + with + " failed"))))
  }

  // rbHash is #hash on a value of unknown type.
  func rbHash(a any) Integer {
    if h, ok := a.(interface{ Hash() Integer }); ok {
      return h.Hash()
    }
    // ponytail: no #hash (Float, Array, plain objects) hashes the inspect
    // text; objects want an identity hash once they have an object id.
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
    return Integer(f)
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
