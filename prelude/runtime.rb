# prelude/runtime.rb
# rbs_inline: enabled
#
# Go helpers that are not methods: output buffering, boxing for T?,
# dispatch on untyped values, exception plumbing.

%x{
  var stdout = bufio.NewWriter(os.Stdout)

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
    return Boolean(any(a) == any(b))
  }

  func rbCmp[T comparable](a, b T) Integer {
    return any(a).(interface{ Cmp(T) Integer }).Cmp(b)
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
