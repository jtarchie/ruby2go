# prelude/json.rb
# rbs_inline: enabled
#
# JSON generation, matching the json gem (2.x): `require "json"` gives every
# object #to_json. Parsing is not supported.

%x{
  type I_ToJson interface{ ToJson(...any) String }

  func rbToJson(a any) String {
    if a == nil {
      return "null"
    }
    if j, ok := a.(I_ToJson); ok {
      return j.ToJson()
    }
    return rbToS(a).ToJson()
  }

  // rbJSONString escapes like the json gem's default generator: quotes,
  // backslashes and control characters; "/" and non-ASCII stay as-is.
  func rbJSONString(s string) String {
    var b strings.Builder
    b.WriteByte('"')
    for i := range len(s) {
      c := s[i]
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
  func rbJSONFloat(f float64) String {
    if math.IsNaN(f) || math.IsInf(f, 0) {
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
  def to_json(*_state) = %x{ return rbToS(self).ToJson() }
end

class String
  #: (*untyped) -> String
  def to_json(*_state) = %x{ rbJSONString(string(self)) }
end

class Symbol
  #: (*untyped) -> String
  def to_json(*_state) = to_s.to_json
end

class Integer
  #: (*untyped) -> String
  def to_json(*_state) = to_s
end

class Float
  #: (*untyped) -> String
  def to_json(*_state) = %x{ rbJSONFloat(float64(self)) }
end

class Boolean
  #: (*untyped) -> String
  def to_json(*_state) = to_s
end

class Array
  #: (*untyped) -> String
  def to_json(*_state) = "[" + map { |x| x.to_json }.join(",") + "]"
end

class Hash
  #: (*untyped) -> String
  def to_json(*_state) = "{" + map { |k, v| k.to_s.to_json + ":" + v.to_json }.join(",") + "}"
end

module JSON
  class GeneratorError < StandardError; end

  #: (untyped) -> String
  def self.generate(obj) = %x{ rbToJson(obj) }
end
