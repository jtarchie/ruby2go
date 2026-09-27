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

  func rbArg[T any](args []any, i int, want string) T {
    if v, ok := args[i].(T); ok {
      return v
    }
    var zero T
    if tup, ok := any(zero).(interface{ _FromAny(any) (T, bool) }); ok { // an Array into a tuple param
      if v, ok := tup._FromAny(args[i]); ok {
        return v
      }
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
