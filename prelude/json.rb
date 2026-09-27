# prelude/json.rb
# rbs_inline: enabled
#
# JSON generation, matching the json gem (2.x): `require "json"` gives every
# object #to_json. Parsing is not supported.

%x{
  type I_ToJson interface{ ToJson(...any) String }

  // rbToJson is `a.to_json(*args)` on any value. The generator passes its
  // state as the one argument, as the gem does, so a user to_json with a
  // signature other than (*untyped) is reached through its Dyn wrapper.
  func rbToJson(a any, args ...any) String {
    a = rbUnbox(a)
    if a == nil {
      return "null"
    }
    if j, ok := a.(I_ToJson); ok {
      return j.ToJson(args...)
    }
    if j, ok := a.(interface{ DynToJson(...any) any }); ok {
      r := j.DynToJson(args...)
      if s, ok := r.(String); ok {
        return s
      }
      panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(r) + " (expected String)"))))
    }
    return rbToS(a).ToJson(args...)
  }

  // rbJSONState is the gem's generator State. It is handed to every
  // nested to_json, which is how depth reaches the indentation.
  type rbJSONState struct {
    indent, space, spaceBefore, objectNl, arrayNl string
    scriptSafe, asciiOnly, allowNaN               bool
    depth                                         int
  }

  // rbJSONStateOf is State.from_state on a to_json's arguments: the state
  // passed down, a new one configured by an options Hash, or a default.
  func rbJSONStateOf(args []any) *rbJSONState {
    st := &rbJSONState{}
    if len(args) == 0 {
      return st
    }
    switch o := rbUnbox(args[0]).(type) {
    case *rbJSONState:
      return o
    case Hash_Any:
      opts := o._ToAny()
      for _, k := range opts.keys {
        v := opts.vals[k]
        switch k {
        case Symbol("indent"):
          st.indent = rbJSONOpt(v)
        case Symbol("space"):
          st.space = rbJSONOpt(v)
        case Symbol("space_before"):
          st.spaceBefore = rbJSONOpt(v)
        case Symbol("object_nl"):
          st.objectNl = rbJSONOpt(v)
        case Symbol("array_nl"):
          st.arrayNl = rbJSONOpt(v)
        case Symbol("script_safe"), Symbol("escape_slash"):
          st.scriptSafe = rbTruthy(v)
        case Symbol("ascii_only"):
          st.asciiOnly = rbTruthy(v)
        case Symbol("allow_nan"):
          st.allowNaN = rbTruthy(v)
        case Symbol("depth"):
          if d, ok := v.(Integer); ok {
            st.depth = int(d)
          }
        case Symbol("sort_keys"), Symbol("strict"), Symbol("as_json"):
          if rbTruthy(v) {
            panic(NewNotImplementedError(Ref(String("JSON generator option " + string(k.(Symbol)) + " is not supported"))))
          }
        }
      }
    }
    return st
  }

  // rbJSONOpt reads a String option; nil means "".
  func rbJSONOpt(v any) string {
    switch s := v.(type) {
    case nil:
      return ""
    case String:
      return string(s)
    }
    panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(v) + " (expected String)"))))
  }

  // rbJSONArray and rbJSONHash port the gem's generate_json_array and
  // generate_json_object: indent per depth before each element, the
  // newline string after the opener and each ",", and before the closer
  // (then indented) only when set.
  func rbJSONArray[E any](xs []E, args []any) String {
    st := rbJSONStateOf(args)
    if len(xs) == 0 {
      return "[]"
    }
    st.depth++
    depth := st.depth
    var b strings.Builder
    b.WriteByte('[')
    b.WriteString(st.arrayNl)
    for i, x := range xs {
      if i > 0 {
        b.WriteByte(',')
        b.WriteString(st.arrayNl)
      }
      b.WriteString(strings.Repeat(st.indent, depth))
      b.WriteString(string(rbToJson(x, st)))
      st.depth = depth
    }
    st.depth--
    if st.arrayNl != "" {
      b.WriteString(st.arrayNl)
      b.WriteString(strings.Repeat(st.indent, st.depth))
    }
    b.WriteByte(']')
    return String(b.String())
  }

  func rbJSONHash[K, V comparable](h *Hash[K, V], args []any) String {
    st := rbJSONStateOf(args)
    if len(h.keys) == 0 {
      return "{}"
    }
    st.depth++
    depth := st.depth
    var b strings.Builder
    b.WriteByte('{')
    for i, k := range h.keys {
      if i > 0 {
        b.WriteByte(',')
      }
      b.WriteString(st.objectNl)
      b.WriteString(strings.Repeat(st.indent, depth))
      b.WriteString(string(rbJSONString(string(rbToS(k)), st)))
      b.WriteString(st.spaceBefore)
      b.WriteByte(':')
      b.WriteString(st.space)
      b.WriteString(string(rbToJson(h.vals[k], st)))
      st.depth = depth
    }
    st.depth--
    if st.objectNl != "" {
      b.WriteString(st.objectNl)
      b.WriteString(strings.Repeat(st.indent, st.depth))
    }
    b.WriteByte('}')
    return String(b.String())
  }

  // rbJSONString escapes like the json gem's generator: quotes,
  // backslashes and control characters; "/" and non-ASCII stay as-is
  // unless st asks for script_safe (also U+2028/9) or ascii_only.
  // Invalid UTF-8 is the gem's GeneratorError.
  func rbJSONString(s string, st *rbJSONState) String {
    if !utf8.ValidString(s) {
      panic(NewJSON_GeneratorError(Ref(String("source sequence is illegal/malformed utf-8"))))
    }
    var b strings.Builder
    b.WriteByte('"')
    for i, r := range s {
      if st.scriptSafe && (r == '/' || r == 0x2028 || r == 0x2029) || st.asciiOnly && r >= 0x80 {
        switch {
        case r == '/':
          b.WriteString(`\\/`)
        case r > 0xffff:
          r1, r2 := utf16.EncodeRune(r)
          fmt.Fprintf(&b, `\\u%04x\\u%04x`, r1, r2)
        default:
          fmt.Fprintf(&b, `\\u%04x`, r)
        }
        continue
      }
      c := s[i]
      if r >= 0x80 {
        b.WriteRune(r)
        continue
      }
      switch c {
      case '"':
        b.WriteString(`\\"`)
      case '\\\\':
        b.WriteString(`\\\\`)
      case '\\n':
        b.WriteString(`\\n`)
      case '\\r':
        b.WriteString(`\\r`)
      case '\\t':
        b.WriteString(`\\t`)
      case '\\b':
        b.WriteString(`\\b`)
      case '\\f':
        b.WriteString(`\\f`)
      default:
        if c < 0x20 {
          fmt.Fprintf(&b, `\\u%04x`, c)
        } else {
          b.WriteByte(c)
        }
      }
    }
    b.WriteByte('"')
    return String(b.String())
  }

  // rbJSONFloat ports the json gem's fpconv emit_digits: shortest digits,
  // plain decimal for moderate exponents, otherwise d.ddde[+-]N.
  func rbJSONFloat(f float64, st *rbJSONState) String {
    if math.IsNaN(f) || math.IsInf(f, 0) {
      if st.allowNaN {
        return rbFloatToS(f)
      }
      panic(NewJSON_GeneratorError(Ref(String(string(rbFloatToS(f)) + " not allowed in JSON"))))
    }
    sign := ""
    if math.Signbit(f) {
      sign = "-"
      f = -f
    }
    if f == 0 {
      return String(sign + "0.0")
    }
    sci := strconv.FormatFloat(f, 'e', -1, 64)
    mant, expStr, _ := strings.Cut(sci, "e")
    digits := strings.Replace(mant, ".", "", 1)
    e, _ := strconv.Atoi(expStr)
    nd := len(digits)
    k := e - nd + 1
    exp := e
    if exp < 0 {
      exp = -exp
    }
    switch {
    case k >= 0 && exp < 15:
      return String(sign + digits + strings.Repeat("0", k) + ".0")
    case k < 0 && (k > -7 || exp < 10):
      offset := nd + k
      if offset <= 0 {
        return String(sign + "0." + strings.Repeat("0", -offset) + digits)
      }
      return String(sign + digits[:offset] + "." + digits[offset:])
    }
    out := sign + digits[:1]
    if nd > 1 {
      out += "." + digits[1:]
    }
    esign := "+"
    if e < 0 {
      esign = "-"
    }
    return String(out + "e" + esign + strconv.Itoa(exp))
  }
}

module Kernel
  # json's Object#to_json: the JSON string of to_s.
  #: (*untyped) -> String
  def to_json(*state) = %x{ return rbToS(self).ToJson(state_...) }
end

class String
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONString(string(self), rbJSONStateOf(state_)) }
end

class Symbol
  #: (*untyped) -> String
  def to_json(*state) = %x{ self.ToS().ToJson(state_...) }
end

class Integer
  #: (*untyped) -> String
  def to_json(*_state) = to_s
end

class Float
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONFloat(float64(self), rbJSONStateOf(state_)) }
end

class Boolean
  #: (*untyped) -> String
  def to_json(*_state) = to_s
end

class Array
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONArray(*self, state_) }
end

class Hash
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONHash(self, state_) }
end

module JSON
  class GeneratorError < StandardError; end

  #: (untyped) -> String
  def self.generate(obj) = %x{ rbToJson(obj, rbJSONStateOf(nil)) }
end
