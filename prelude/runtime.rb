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
  // MRI, and SystemExit (Kernel#exit) into its status. Output is flushed
  // first so partial output before a crash matches.
  func rbTopRecover() {
    if r := recover(); r != nil {
      _ = stdout.Flush()
      if e, ok := r.(SystemExitI); ok {
        os.Exit(int(e.Status()))
      }
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

  // rbObjInspect is Kernel#inspect; rbInspectEnter gives MRI's "..." for an object that holds itself.
  func rbObjInspect(a any) String {
    o, ok := a.(interface{ _Ivars() []rbIvar })
    if !ok {
      return rbObjToS(a)
    }
    s := string(rbObjToS(a))
    var b strings.Builder
    b.WriteString(s[:len(s)-1])
    if !rbInspectEnter(a) {
      return String(b.String() + " ...>")
    }
    defer rbInspectLeave(a)
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
    if e, ok := x.(interface{ _EqAny(any) Boolean }); ok {
      return e._EqAny(y) // a == typed on its argument; see emitEqAdapter
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

  // rbCmp is <=> for sort, min and max. Typed values have Op_cmp(T);
  // T? boxes compare their values (rbCmpBox); untyped ones (T is any) go
  // through the generated DynOp_cmp wrappers (rbCmpFailed), which answer
  // nil for an incomparable argument, as MRI's <=> does.
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

  // rbCmpFailed is <=> where no Op_cmp(T) applies: an untyped value asks
  // its DynOp_cmp wrapper, and a nil answer raises MRI's rb_cmperr.
  func rbCmpFailed(a, b any) Integer {
    a, b = rbUnbox(a), rbUnbox(b)
    if c, ok := a.(interface{ DynOp_cmp(...any) any }); ok {
      if r, ok := c.DynOp_cmp(b).(Integer); ok {
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

  func rbIdentical(a, b any) bool {
    if as, ok := a.(String); ok {
      bs, ok := b.(String)
      // Identity of immutable strings is their backing pointer.
      return ok && len(as) == len(bs) && unsafe.StringData(string(as)) == unsafe.StringData(string(bs)) //nolint:gosec // pointer compare only
    }
    return a == b
  }

  // rbNewStr is a String method's result, a new object in MRI: when it is
  // the receiver's own bytes (nothing to strip, replace or pad), it is
  // copied, so equal? and frozen? don't take it for the receiver. Only that
  // case pays for the copy.
  // ponytail: other shared bytes still look identical (equal slices of one string, strconv's small-number table, a "" Go boxes to zeroVal); a boxed String would fix them.
  func rbNewStr(recv, s String) String {
    if len(s) == len(recv) && unsafe.StringData(string(s)) == unsafe.StringData(string(recv)) { //nolint:gosec // pointer compare only
      return rbStrClone(s)
    }
    return s
  }

  // rbStrClone copies s to bytes of its own; "" gets an address of its own.
  func rbStrClone(s String) String {
    if s == "" {
      return String(unsafe.String(new(byte), 0)) //nolint:gosec // an address for identity only
    }
    return String(strings.Clone(string(s)))
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

  // rbFloatPow is Float#**. MRI calls C's pow(), which glibc and macOS round
  // correctly bar inputs a hair from a rounding midpoint; Go's math.Pow is
  // often an ulp or more off (8.0 ** (1.0/3) is 1.9999999999999998, MRI
  // 2.0). This evaluates exp(y*log|x|) in double-double (error under 2^-84)
  // and rounds once. math.Pow keeps the special values, which it treats as
  // C99's pow does, and results past the normal range.
  // ponytail: subnormal results can be an ulp off; round them at 2^-1074 by adding 1.0 first (musl's exp specialcase) if that matters.
  // ponytail: costs 5-10x math.Pow; square-and-multiply in double-double would make small integer exponents cheap.
  func rbFloatPow(x, y float64) float64 {
    const ln2lo = 0x1.abc9e3b39803fp-56 // math.Ln2 - float64(math.Ln2)
    switch {
    case y == 2: // MRI's own shortcut; one rounding already
      return x * x
    case x == 0 || y == 0 || x == 1 || x == -1 || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0),
      x < 0 && y != math.Trunc(y):
      return math.Pow(x, y)
    }
    // log|x| = e*ln2 + 2s*sum(t^n/(2n+1)), with s = (f-1)/(f+1), t = s*s < 0.03
    // for f in [sqrt(1/2), sqrt(2)); f-1 is exact.
    f, e := math.Frexp(math.Abs(x))
    if f < math.Sqrt2/2 {
      f, e = 2*f, e-1
    }
    dh, dl := rbTwoSum(f, 1)
    sh := (f - 1) / dh
    sl := (math.FMA(-sh, dh, f-1) - sh*dl) / dh
    th, tl := rbDDMul(sh, sl, sh, sl)
    ah, al := 0.0, 0.0
    for n := 18; n >= 0; n-- {
      c := rbPowInv[n]
      if n > 6 { // terms under 2^-39 need no low word
        ah = ah*th + c[0]
        continue
      }
      ah, al = rbDDMul(ah, al, th, tl)
      ah, al = rbDDAdd(ah, al, c[0], c[1])
    }
    lh, ll := rbDDMul(sh, sl, 2*ah, 2*al)
    fe := float64(e)
    kh := float64(fe * math.Ln2)
    lh, ll = rbDDAdd(kh, math.FMA(fe, math.Ln2, -kh)+fe*ln2lo, lh, ll)
    // x**y = 2^k * exp(r), r = y*log|x| - k*ln2, |r| <= ln2/2
    ph := float64(y * lh)
    pl := math.FMA(y, lh, -ph) + y*ll
    if !(ph > -708.39 && ph < 709.79) { // subnormal or overflowing
      return math.Pow(x, y)
    }
    k := math.Round(ph * math.Log2E)
    kh = float64(k * math.Ln2)
    rh, rl := rbDDAdd(ph, pl, -kh, -math.FMA(k, math.Ln2, -kh)-k*ln2lo)
    // expm1(r/4) by Taylor, then expm1(2a) = expm1(a)*(expm1(a)+2) twice.
    rh, rl = rh/4, rl/4
    eh, el := 0.0, 0.0
    for n := 16; n >= 1; n-- {
      c := rbPowFact[n]
      if n > 7 { // terms under 2^-39 of the first
        eh = (eh + c[0]) * rh
        continue
      }
      eh, el = rbDDAdd(eh, el, c[0], c[1])
      eh, el = rbDDMul(eh, el, rh, rl)
    }
    for range 2 {
      mh, ml := rbDDAdd(eh, el, 2, 0)
      eh, el = rbDDMul(eh, el, mh, ml)
    }
    res, _ := rbDDAdd(1, 0, eh, el)
    res = math.Ldexp(res, int(k))
    if x < 0 && math.Mod(y, 2) != 0 {
      return -res
    }
    return res
  }

  // rbPowInv[n] is 1/(2n+1) and rbPowFact[n] is 1/n!, as double-doubles.
  var rbPowInv, rbPowFact = func() (inv, fact [19][2]float64) {
    nf := 1.0
    for n := range inv {
      d := float64(2*n + 1)
      if n > 0 {
        nf *= float64(n)
      }
      inv[n] = [2]float64{1 / d, math.FMA(-1/d, d, 1) / d}
      fact[n] = [2]float64{1 / nf, math.FMA(-1/nf, nf, 1) / nf}
    }
    return inv, fact
  }()

  // Double-double arithmetic: a value is hi+lo with |lo| <= ulp(hi)/2. The
  // float64() around a product stops Go fusing it into a later add (an FMA),
  // which would break the error-free split.
  func rbTwoSum(a, b float64) (float64, float64) {
    s := a + b
    bb := s - a
    return s, (a - (s - bb)) + (b - bb)
  }

  func rbDDAdd(ah, al, bh, bl float64) (float64, float64) {
    s, e := rbTwoSum(ah, bh)
    e += al + bl
    h := s + e
    return h, e - (h - s)
  }

  func rbDDMul(ah, al, bh, bl float64) (float64, float64) {
    p := float64(ah * bh)
    e := math.FMA(ah, bh, -p) + (ah*bl + al*bh)
    h := p + e
    return h, e - (h - p)
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
