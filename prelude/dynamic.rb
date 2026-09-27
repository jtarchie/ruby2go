# prelude/dynamic.rb
# rbs_inline: enabled
#
# Runtime support for calls on untyped values. The transpiler generates, per
# method name called that way, a `DynName(args ...any) any` wrapper on each
# class having the method, and an `rbDynName` dispatcher: the wrapper, else
# method_missing, else NoMethodError. These helpers check arity and convert
# arguments the way a typed call would have.

%x{
  // rbArity raises MRI's ArgumentError for a wrong argument count.
  func rbArity(given, min, max int) {
    if given >= min && (max < 0 || given <= max) {
      return
    }
    expected := strconv.Itoa(min)
    switch {
    case max < 0:
      expected += "+"
    case max != min:
      expected += ".." + strconv.Itoa(max)
    }
    panic(NewArgumentError(Ref(String(fmt.Sprintf("wrong number of arguments (given %d, expected %s)", given, expected)))))
  }

  // rbAs converts v to T, widening an Integer where a Float is expected
  // (MRI's coerce: Float's operators take Integers).
  func rbAs[T any](v any) (T, bool) {
    if t, ok := v.(T); ok {
      return t, true
    }
    if n, ok := v.(Integer); ok {
      t, ok := any(Float(n)).(T)
      return t, ok
    }
    var zero T
    return zero, false
  }

  // rbNumMixed reports an Integer or Float argument of the other class than
  // self (an Integer or a Float).
  func rbNumMixed(self any, args []any) bool {
    _, selfInt := self.(Integer)
    for _, a := range args {
      switch a.(type) {
      case Integer:
        if !selfInt {
          return true
        }
      case Float:
        if selfInt {
          return true
        }
      }
    }
    return false
  }

  // rbNum is an Integer or a Float as one Go type: Comparable's methods run
  // on it when given both, so clamp returns the winning argument itself.
  type rbNum struct{ v any }

  func (a rbNum) Cmp(b rbNum) Integer { return rbCmp(a.v, b.v) }

  func (a rbNum) Lt(b rbNum) Boolean { return rbCmp(a.v, b.v) < 0 }

  func rbArg[T any](args []any, i int, want string) T {
    if v, ok := rbAs[T](args[i]); ok {
      return v
    }
    panic(NewTypeError(Ref(String("no implicit conversion of " + rbDescribe(args[i]) + " into " + want))))
  }

  func rbOptArg[T any](args []any, i int, want string) *T {
    if args[i] == nil {
      return nil
    }
    v := rbArg[T](args, i, want)
    return &v
  }

  func rbRest[T any](args []any, from int, want string) []T {
    out := make([]T, 0, len(args))
    for i := from; i < len(args); i++ {
      out = append(out, rbArg[T](args, i, want))
    }
    return out
  }

  // rbDescribe names a value the way MRI's messages do.
  func rbDescribe(v any) string {
    switch r := v.(type) {
    case nil:
      return "nil"
    case Boolean:
      return strconv.FormatBool(bool(r))
    case rbModule:
      return r._Kind() + " " + string(r.Name())
    }
    return "an instance of " + rbClassName(v)
  }

  // rbNoMethod is MRI's error for a missing method; a bare `name` (a
  // "vcall": no receiver, arguments or parentheses) is a NameError.
  func rbNoMethod(name string, recv any, vcall bool) any {
    if vcall {
      return NewNameError(Ref(String("undefined local variable or method '" + name + "' for " + rbDescribe(recv))))
    }
    return NewNoMethodError(Ref(String("undefined method '" + name + "' for " + rbDescribe(recv))))
  }
}
