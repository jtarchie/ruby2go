# prelude/string.rb
# rbs_inline: enabled
#
# String as a named Go string (frozen: no mutating methods).

# @go_type string
class String < Object
  include Comparable

  #: (String) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  # == on two Strings (decision 12's class overload): a plain Go compare,
  # inlined, with no boxing of the argument into untyped.
  #: (String) -> bool
  def __eq_string(other) = %x{ Boolean(self == other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(String)
    return Boolean(ok && self == o)
  }

  #: (String) -> String
  def +(other) = %x{ rbNewStr(self, rbNewStr(other, self + other)) }

  #: (Integer) -> String
  def *(n) = %x{
    if n < 0 {
      panic(NewArgumentError(Ref[String]("negative argument")))
    }
    return rbNewStr(self, String(strings.Repeat(string(self), int(n))))
  }

  #: () -> String
  def to_s = self

  #: () -> String
  def to_str = self

  #: () -> String
  def inspect = %x{ rbStringInspect(string(self)) }

  #: () -> String
  def dup = %x{ rbStrClone(self) }

  # `+s` is s when not frozen, else an unfrozen copy; `-s` is s when
  # frozen, else a frozen copy (MRI also dedups it).
  #: () -> String
  def +@ = frozen? ? dup : self

  #: () -> String
  def -@ = frozen? ? self : dup.freeze

  #: () -> String
  def upcase = %x{ rbNewStr(self, String(rbCaseMap(string(self), 2, unicode.ToUpper))) }

  #: () -> String
  def downcase = %x{ rbNewStr(self, String(rbCaseMap(string(self), 0, unicode.ToLower))) }

  # Titlecase the first character (ǆ -> ǅ, ß -> Ss), downcase the rest.
  #: () -> String
  def capitalize = %x{
    s := string(self)
    if s == "" {
      return ""
    }
    r, n := utf8.DecodeRuneInString(s)
    title := unicode.ToTitle
    if r >= 0x1C90 && r <= 0x1CBF { // Georgian Mtavruli: MRI titlecases to Mkhedruli, its lowercase
      title = unicode.ToLower
    }
    return String(rbCaseMap(s[:n], 1, title) + rbCaseMap(s[n:], 0, unicode.ToLower))
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
  def strip = %x{ rbNewStr(self, String(strings.Trim(string(self), " \\t\\n\\v\\f\\r\\x00"))) }

  #: () -> String
  def lstrip = %x{ rbNewStr(self, String(strings.TrimLeft(string(self), " \\t\\n\\v\\f\\r\\x00"))) }

  #: () -> String
  def rstrip = %x{ rbNewStr(self, String(strings.TrimRight(string(self), " \\t\\n\\v\\f\\r\\x00"))) }

  #: () -> String
  def chomp = %x{ rbNewStr(self, String(strings.TrimSuffix(strings.TrimSuffix(string(self), "\\n"), "\\r"))) }

  # chomp(suffix): that suffix once ("" is MRI's paragraph mode: every trailing newline).
  #: (String) -> String
  def __chomp_1(suffix) = %x{
    if suffix == "" {
      return String(strings.TrimRight(string(self), "\\r\\n"))
    }
    return String(strings.TrimSuffix(string(self), string(suffix)))
  }

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

  #: (String, String) -> bool
  def __start_with_q_2(a, b) = start_with?(a) || start_with?(b)

  #: (String, String, String) -> bool
  def __start_with_q_3(a, b, c) = start_with?(a) || start_with?(b) || start_with?(c)

  #: (String, String) -> bool
  def __end_with_q_2(a, b) = end_with?(a) || end_with?(b)

  #: (String, String, String) -> bool
  def __end_with_q_3(a, b, c) = end_with?(a) || end_with?(b) || end_with?(c)

  #: (String) -> String
  def delete_prefix(s) = %x{ rbNewStr(self, String(strings.TrimPrefix(string(self), string(s)))) }

  #: (String) -> String
  def delete_suffix(s) = %x{ rbNewStr(self, String(strings.TrimSuffix(string(self), string(s)))) }

  #: (String) -> [String, String, String]
  def partition(sep) = %x{
    before, after, ok := strings.Cut(string(self), string(sep))
    if !ok {
      return Tuple3[String, String, String]{rbNewStr(self, self), "", ""}
    }
    return Tuple3[String, String, String]{String(before), rbNewStr(sep, sep), String(after)}
  }

  #: (String) -> [String, String, String]
  def rpartition(sep) = %x{
    i := strings.LastIndex(string(self), string(sep))
    if i < 0 {
      return Tuple3[String, String, String]{"", "", rbNewStr(self, self)}
    }
    return Tuple3[String, String, String]{self[:i], rbNewStr(sep, sep), self[i+len(sep):]}
  }

  #: (String) -> bool
  def include?(s) = %x{ Boolean(strings.Contains(string(self), string(s))) }

  # Strings are values in rb2go (decision 136): a new one is its argument's bytes.
  #: (?String) -> String
  def self.new(s = "") = s

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

  #: (Range[Integer]) -> String?
  def __idx_range(r)
    s = r.__slice(length)
    return nil unless s
    __idx_2(s[0], s[1])
  end

  #: (Integer, Integer) -> String?
  def __idx_2(start, count) = %x{
    r := []rune(string(self))
    n := Integer(len(r))
    if start < 0 {
      start += n
    }
    if start < 0 || start > n || count < 0 {
      return nil
    }
    end := min(start+count, n)
    return Ref(rbNewStr(self, String(r[start:end])))
  }

  #: () -> Array[Integer]
  def bytes = %x{
    out := &Array[Integer]{s: make([]Integer, len(self))}
    for i := range len(self) {
      out.s[i] = Integer(self[i])
    }
    return out
  }

  #: () -> Array[String]
  def chars = %x{
    out := &Array[String]{}
    for _, r := range string(self) {
      out.s = append(out.s, String(r))
    }
    return out
  }

  #: () -> Enumerator[String]
  def __each_char_enum = %x{ return rbEnumOf(self.EachChar(), any(self), "each_char", rbSizeOf(Integer(utf8.RuneCountInString(string(self)))), nil) }

  #: () { (String) -> void } -> void
  def each_line
    lines.each { |l| yield l }
  end

  #: () -> Enumerator[String]
  def __each_line_enum = %x{ return rbEnumOf(Array_Each(self.Lines()), any(self), "each_line", nil, nil) }

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
        out.s = append(out.s, rbNewStr(self, String(l)))
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
        out.s = append(out.s, rbNewStr(self, String(f)))
      }
      return out
    }
    parts := strings.Split(string(self), string(*sep))
    for len(parts) > 0 && parts[len(parts)-1] == "" {
      parts = parts[:len(parts)-1]
    }
    for _, p := range parts {
      out.s = append(out.s, rbNewStr(self, String(p)))
    }
    return out
  }

  #: (String, String) -> String
  def sub(from, to) = %x{ rbNewStr(self, String(rbSub(string(self), string(from), string(to), 1))) }

  #: (String, String) -> String
  def gsub(from, to) = %x{ rbNewStr(self, String(rbSub(string(self), string(from), string(to), -1))) }

  #: (String, String) -> String
  def tr(from, to) = %x{
    // ponytail: a reversed range ("z-a") expands to nothing; MRI raises ArgumentError.
    expand := func(s []rune) []rune {
      var out []rune
      for i := 0; i < len(s); i++ {
        switch {
        case s[i] == '\\\\' && i+1 < len(s):
          i++
          out = append(out, s[i])
        case i+2 < len(s) && s[i+1] == '-':
          for r := s[i]; r <= s[i+2]; r++ {
            out = append(out, r)
          }
          i += 2
        default:
          out = append(out, s[i])
        }
      }
      return out
    }
    f := []rune(string(from))
    negate := len(f) > 1 && f[0] == '^'
    if negate {
      f = f[1:]
    }
    f, t := expand(f), expand([]rune(string(to)))
    // An empty to-list deletes; otherwise it pads with its last rune.
    last := rune(-1)
    if len(t) > 0 {
      last = t[len(t)-1]
    }
    m := make(map[rune]rune, len(f))
    for i, c := range f {
      m[c] = last
      if !negate && i < len(t) {
        m[c] = t[i]
      }
    }
    return rbNewStr(self, String(strings.Map(func(r rune) rune {
      v, ok := m[r]
      if negate {
        if ok {
          return r
        }
        return last
      }
      if ok {
        return v
      }
      return r
    }, string(self))))
  }

  # An Array argument supplies every directive's value, as MRI's.
  #: (untyped) -> String
  def %(arg) = %x{
    if a, ok := rbUnbox(arg).(Array_Any); ok {
      return String(rbFormat(string(self), a._ToAny().s))
    }
    return String(rbFormat(string(self), []any{arg}))
  }

  # Base 0 reads a 0b/0o/0x prefix (or a bare leading 0 as octal); a
  # prefix matching the base is skipped. Invalid digits end the number.
  #: (Integer) -> Integer
  def __to_i_1(base) = %x{ Integer(rbStrToIBase(string(self), int(base))) }

  #: () -> Integer
  def hex = %x{ Integer(rbStrToIBase(string(self), 16)) }

  # Octal, unless a 0b/0o/0x prefix says otherwise.
  #: () -> Integer
  def oct = %x{ Integer(rbStrToIBase(string(self), -8)) }

  # MRI's String#succ: the rightmost alphanumeric increments with carry
  # (a digit to a digit, a letter to a letter of its case), growing on the
  # left; without alphanumerics the rightmost character increments.
  #: () -> String
  def succ = %x{ String(rbStrSucc(string(self))) }

  #: () -> String
  def next = succ

  # One character set only (no intersection of several, as MRI's).
  #: (String) -> Integer
  def count(chars) = %x{
    in := rbCharSet(string(chars))
    n := 0
    for _, r := range string(self) {
      if in(r) {
        n++
      }
    }
    return Integer(n)
  }

  #: () -> String
  def squeeze = %x{ String(rbSqueeze(string(self), func(rune) bool { return true })) }

  #: (String) -> String
  def __squeeze_1(chars) = %x{ String(rbSqueeze(string(self), rbCharSet(string(chars)))) }

  #: () -> String
  def swapcase = %x{
    return String(strings.Map(func(r rune) rune {
      switch {
      case unicode.IsUpper(r):
        return unicode.ToLower(r)
      case unicode.IsLower(r):
        return unicode.ToUpper(r)
      }
      return r
    }, string(self)))
  }

  # ASCII-only case folding, as MRI's casecmp.
  #: (String) -> Integer
  def casecmp(other) = %x{
    lower := func(s string) string {
      return strings.Map(func(r rune) rune {
        if r >= 'A' && r <= 'Z' {
          return r + 32
        }
        return r
      }, s)
    }
    return Integer(strings.Compare(lower(string(self)), lower(string(other))))
  }

  #: (String) -> bool
  def casecmp?(other) = %x{ Boolean(strings.EqualFold(string(self), string(other))) }

  # Drops the last character; a trailing \r\n goes as one.
  #: () -> String
  def chop = %x{
    s := string(self)
    if strings.HasSuffix(s, "\\r\\n") {
      return String(s[:len(s)-2])
    }
    _, w := utf8.DecodeLastRuneInString(s)
    return String(s[:len(s)-w])
  }

  #: () -> String
  def chr = %x{
    _, w := utf8.DecodeRuneInString(string(self))
    return self[:w]
  }

  #: () -> bool
  def ascii_only? = %x{
    for i := range len(self) {
      if self[i] >= 0x80 {
        return false
      }
    }
    return true
  }

  # A positive limit caps the field count, the last field keeping the rest;
  # a negative one keeps trailing empty fields.
  #: (String?, Integer) -> Array[String]
  def __split_2(sep, limit) = %x{
    out := &Array[String]{}
    for _, p := range rbSplitLimit(string(self), sep, int(limit)) {
      out.s = append(out.s, String(p))
    }
    return out
  }

  #: (Integer, Integer) -> String?
  def slice(start, len) = self[start, len]

  # One character set only (no intersection of several, as MRI's).
  #: (String) -> String
  def delete(chars) = tr(chars, "")

  #: () -> Integer
  def to_i = %x{
    m := rbIntPrefix.FindStringSubmatch(string(self))
    if m == nil {
      return 0
    }
    digits := m[1] + strings.ReplaceAll(m[2], "_", "")
    n, err := strconv.Atoi(digits)
    if errors.Is(err, strconv.ErrRange) { // MRI returns a Bignum (decision 35)
      panic(NewRangeError(Ref(String(digits + " overflows Integer (64-bit; no Bignum)"))))
    }
    return Integer(n)
  }

  #: () -> Float
  def to_f = %x{
    m := rbFloatPrefix.FindStringSubmatch(string(self))
    if m == nil {
      return 0
    }
    f, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], "_", ""), 64)
    return Float(f)
  }

  #: () -> Integer
  def ord = %x{
    if self == "" {
      panic(NewArgumentError(Ref[String]("empty string")))
    }
    r, n := utf8.DecodeRuneInString(string(self))
    if r == utf8.RuneError && n == 1 {
      return Integer(self[0]) // a binary byte, e.g. 200.chr
    }
    return Integer(r)
  }

  #: () -> Integer
  def hash = %x{
    h := fnv.New64a()
    _, _ = h.Write([]byte(self))
    return Integer(h.Sum64())
  }

  #: () -> String
  def freeze = %x{
    rbStrFrozen(string(self), true)
    return self
  }

  # Literals are frozen (frozen_string_literal); strings built at run time
  # are not until frozen. See rbStrFrozen.
  #: () -> bool
  def frozen? = %x{ Boolean(rbStrFrozen(string(self), false)) }

  #: (Integer, ?String) -> String
  def center(width, pad = " ") = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return rbStrClone(self)
    }
    return String(rbPad(string(pad), n/2) + string(self) + rbPad(string(pad), n-n/2))
  }

  #: (Integer, ?String) -> String
  def ljust(width, pad = " ") = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return rbStrClone(self)
    }
    return self + String(rbPad(string(pad), n))
  }

  #: (Integer, ?String) -> String
  def rjust(width, pad = " ") = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return rbStrClone(self)
    }
    return String(rbPad(string(pad), n)) + self
  }

  #: (String) -> Integer?
  def rindex(s) = %x{
    i := strings.LastIndex(string(self), string(s))
    if i < 0 {
      return nil
    }
    return Ref(Integer(utf8.RuneCountInString(string(self)[:i])))
  }

  #: (String) -> Integer?
  def byteindex(s) = %x{
    i := strings.Index(string(self), string(s))
    if i < 0 {
      return nil
    }
    return Ref(Integer(i))
  }

  #: (String) -> Integer?
  def byterindex(s) = %x{
    i := strings.LastIndex(string(self), string(s))
    if i < 0 {
      return nil
    }
    return Ref(Integer(i))
  }

  # One byte without a length, as MRI (byteslice(3) of "abc" is nil, byteslice(3, 1) is "").
  #: (Integer, ?Integer?) -> String?
  def byteslice(start, count = nil) = %x{
    n := Integer(len(self))
    if start < 0 {
      start += n
    }
    if count == nil {
      if start < 0 || start >= n {
        return nil
      }
      return Ref(String(self[start : start+1]))
    }
    if start < 0 || start > n || *count < 0 {
      return nil
    }
    return Ref(String(strings.Clone(string(self[start:min(start+*count, n)]))))
  }

  #: (Integer) -> Integer?
  def getbyte(i) = %x{
    if i < 0 {
      i += Integer(len(self))
    }
    if i < 0 || int(i) >= len(self) {
      return nil
    }
    return Ref(Integer(self[i]))
  }

  #: () -> Array[Integer]
  def codepoints = %x{
    out := &Array[Integer]{}
    for _, r := range string(self) {
      out.s = append(out.s, Integer(r))
    }
    return out
  }

  #: () { (Integer) -> void } -> void
  def each_byte = %x{
    return func(yield func(Integer) bool) {
      for i := range len(self) {
        if !yield(Integer(self[i])) {
          return
        }
      }
    }
  }

  #: () { (Integer) -> void } -> void
  def each_codepoint = %x{
    return func(yield func(Integer) bool) {
      for _, r := range string(self) {
        if !yield(Integer(r)) {
          return
        }
      }
    }
  }

  #: (?Integer) -> Integer
  def sum(bits = 16) = %x{
    var t uint64
    for i := range len(self) {
      t += uint64(self[i])
    }
    if bits > 0 && bits < 64 {
      t &= 1<<bits - 1
    }
    return Integer(t)
  }

  #: (String) { (String) -> void } -> void
  def upto(last)
    if __digits? && last.__digits?
      i = to_i
      while i <= last.to_i
        yield i.to_s.rjust(size, "0")
        i += 1
      end
      return
    end
    return if (self <=> last) > 0
    s = self
    while true
      yield s
      break if s == last
      s = s.succ
      break if s.size > last.size || s.empty?
    end
  end

  #: () -> bool
  def __digits? = %x{ Boolean(self != "" && strings.Trim(string(self), "0123456789") == "") }

  #: () -> Symbol
  def intern = to_sym

  #: () -> String
  def b = %x{ rbStrClone(self) }

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := rbUnbox(other).(String)
    return Boolean(ok && o == self)
  }

  #: () -> String
  def dump = %x{ String(rbStrDump(string(self))) }

  #: () -> String
  def undump = %x{ return rbStrUndump(string(self)) }

  #: (String) -> Array[untyped]
  def unpack(format) = %x{ return rbUnpack(string(self), string(format)) }

  #: (String) -> untyped
  def unpack1(format) = %x{
    if out := rbUnpack(string(self), string(format)).s; len(out) > 0 {
      return out[0]
    }
    return nil
  }
end
