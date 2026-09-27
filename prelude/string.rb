# prelude/string.rb
# rbs_inline: enabled
#
# String as a named Go string (frozen: no mutating methods).

# @go_type string
class String < Object
  include Comparable

  #: (String) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(String)
    return Boolean(ok && self == o)
  }

  #: (String) -> String
  def +(other) = %x{ self + other }

  #: (Integer) -> String
  def *(n) = %x{ String(strings.Repeat(string(self), int(n))) }

  #: () -> String
  def to_s = self

  #: () -> String
  def to_str = self

  #: () -> String
  def inspect = %x{ rbStringInspect(string(self)) }

  #: () -> String
  def dup = %x{ String(strings.Clone(string(self))) }

  #: () -> String
  def upcase = %x{ String(strings.ToUpper(string(self))) }

  #: () -> String
  def downcase = %x{ String(strings.ToLower(string(self))) }

  #: () -> String
  def capitalize = %x{
    s := strings.ToLower(string(self))
    if s == "" {
      return ""
    }
    r, n := utf8.DecodeRuneInString(s)
    return String(string(unicode.ToUpper(r)) + s[n:])
  }

  #: () -> String
  def reverse = %x{
    r := []rune(string(self))
    for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
      r[i], r[j] = r[j], r[i]
    }
    return String(r)
  }

  # MRI's whitespace is ASCII only, plus NUL for strip; not unicode.IsSpace.
  #: () -> String
  def strip = %x{ String(strings.Trim(string(self), " \\t\\n\\v\\f\\r\\x00")) }

  #: () -> String
  def lstrip = %x{ String(strings.TrimLeft(string(self), " \\t\\n\\v\\f\\r\\x00")) }

  #: () -> String
  def rstrip = %x{ String(strings.TrimRight(string(self), " \\t\\n\\v\\f\\r\\x00")) }

  #: () -> String
  def chomp = %x{ String(strings.TrimSuffix(strings.TrimSuffix(string(self), "\\n"), "\\r")) }

  #: () -> Integer
  def size = %x{ Integer(utf8.RuneCountInString(string(self))) }

  #: () -> Integer
  def length = size

  #: () -> Integer
  def bytesize = %x{ Integer(len(self)) }

  #: () -> bool
  def empty? = %x{ self == "" }

  #: (String) -> bool
  def end_with?(s) = %x{ Boolean(strings.HasSuffix(string(self), string(s))) }

  #: (String) -> bool
  def start_with?(s) = %x{ Boolean(strings.HasPrefix(string(self), string(s))) }

  #: (String) -> bool
  def include?(s) = %x{ Boolean(strings.Contains(string(self), string(s))) }

  #: (String) -> Integer?
  def index(s) = %x{
    i := strings.Index(string(self), string(s))
    if i < 0 {
      return nil
    }
    return Ref(Integer(utf8.RuneCountInString(string(self)[:i])))
  }

  #: (Integer) -> String?
  def [](i) = %x{
    r := []rune(string(self))
    if i < 0 {
      i += Integer(len(r))
    }
    if i < 0 || int(i) >= len(r) {
      return nil
    }
    return Ref(String(r[i]))
  }

  #: () -> Array[String]
  def chars = %x{
    out := &Array[String]{}
    for _, r := range string(self) {
      *out = append(*out, String(r))
    }
    return out
  }

  #: () { (String) -> void } -> void
  def each_char = %x{
    return func(yield func(String) bool) {
      for _, r := range string(self) {
        if !yield(String(r)) {
          return
        }
      }
    }
  }

  #: () -> Array[String]
  def lines = %x{
    out := &Array[String]{}
    for _, l := range strings.SplitAfter(string(self), "\\n") {
      if l != "" {
        *out = append(*out, String(l))
      }
    }
    return out
  }

  # Without a separator, or with " " (awk mode): split on ASCII whitespace,
  # dropping empties. With one: split on it, dropping trailing empties, like MRI.
  #: (?String?) -> Array[String]
  def split(sep = nil) = %x{
    out := &Array[String]{}
    if sep == nil || *sep == " " {
      isSpace := func(r rune) bool { return r == ' ' || r >= '\\t' && r <= '\\r' }
      for _, f := range strings.FieldsFunc(string(self), isSpace) {
        *out = append(*out, String(f))
      }
      return out
    }
    parts := strings.Split(string(self), string(*sep))
    for len(parts) > 0 && parts[len(parts)-1] == "" {
      parts = parts[:len(parts)-1]
    }
    for _, p := range parts {
      *out = append(*out, String(p))
    }
    return out
  }

  #: (String, String) -> String
  def sub(from, to) = %x{ String(strings.Replace(string(self), string(from), string(to), 1)) }

  #: (String, String) -> String
  def gsub(from, to) = %x{ String(strings.ReplaceAll(string(self), string(from), string(to))) }

  #: (String, String) -> String
  def tr(from, to) = %x{
    f, t := []rune(string(from)), []rune(string(to))
    return String(strings.Map(func(r rune) rune {
      for i, c := range f {
        if c == r {
          if i < len(t) {
            return t[i]
          }
          return t[len(t)-1]
        }
      }
      return r
    }, string(self)))
  }

  #: () -> Integer
  def to_i = %x{
    s := strings.TrimLeft(string(self), " \\t\\n\\v\\f\\r")
    end := 0
    if end < len(s) && (s[end] == '-' || s[end] == '+') {
      end++
    }
    for end < len(s) && s[end] >= '0' && s[end] <= '9' {
      end++
    }
    n, _ := strconv.Atoi(s[:end])
    return Integer(n)
  }

  #: () -> Float
  def to_f = %x{
    f, _ := strconv.ParseFloat(strings.Trim(string(self), " \\t\\n\\v\\f\\r"), 64)
    return Float(f)
  }

  #: () -> Integer
  def ord = %x{
    r, _ := utf8.DecodeRuneInString(string(self))
    return Integer(r)
  }

  #: () -> Integer
  def hash = %x{
    h := fnv.New64a()
    _, _ = h.Write([]byte(self))
    return Integer(h.Sum64())
  }

  #: () -> String
  def freeze = self

  #: (Integer) -> String
  def center(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return self
    }
    return String(strings.Repeat(" ", n/2) + string(self) + strings.Repeat(" ", n-n/2))
  }

  #: (Integer) -> String
  def ljust(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return self
    }
    return self + String(strings.Repeat(" ", n))
  }

  #: (Integer) -> String
  def rjust(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return self
    }
    return String(strings.Repeat(" ", n)) + self
  }
end
