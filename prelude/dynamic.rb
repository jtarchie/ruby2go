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

  // rbConv is v as a T: v itself, nil for untyped, or an Array or Hash of
  // another instantiation converted (rbFrom). Go instantiations are
  // invariant, so Array[Integer] where Array[untyped] is expected, or
  // back, is a copy whose elements are converted in turn.
  // ponytail: a T? element (*T) is not converted from its value; box it via OptOf when needed.
  func rbConv[T any](v any) (T, bool) {
    if t, ok := v.(T); ok {
      return t, true
    }
    var zero T
    if v == nil {
      _, untyped := any(&zero).(*any)
      return zero, untyped
    }
    if c, ok := any(zero).(interface{ rbFrom(v any) (T, bool) }); ok {
      return c.rbFrom(v)
    }
    return zero, false
  }

  func rbRest[T any](args []any, from int, want string) []T {
    out := make([]T, 0, len(args))
    for i := from; i < len(args); i++ {
      out = append(out, rbAs[T](args[i], want))
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
