# prelude/string.rb
# rbs_inline: enabled
#
# String as a named Go string (frozen: no mutating methods).

%x{
  // The numeric prefix String#to_i / #to_f read, as MRI scans it: leading
  // whitespace, a sign, digits with single `_` between them; to_i takes a
  // `0d` prefix, to_f a fraction and exponent. The rest of the string is ignored.
  var (
    rbIntPrefix   = regexp.MustCompile(`\\A[ \\t\\n\\v\\f\\r]*([+-]?)(?:0[dD])?(\\d+(?:_\\d+)*)`)
    rbFloatPrefix = regexp.MustCompile(`\\A[ \\t\\n\\v\\f\\r]*([+-]?(?:\\d+(?:_\\d+)*(?:\\.(?:\\d+(?:_\\d+)*)?)?|\\.\\d+(?:_\\d+)*)(?:[eE][+-]?\\d+(?:_\\d+)*)?)`)
  )

  // SpecialCasing.txt's unconditional mappings, which MRI applies and Go's
  // per-rune unicode.To* cannot (one rune to several): lower, title, upper.
  var rbSpecialCasing = map[rune][3]string{
    'ß': {"ß", "Ss", "SS"}, 'İ': {"i\u0307", "İ", "İ"}, 'ŉ': {"ŉ", "ʼN", "ʼN"},
    'ǰ': {"ǰ", "J\u030C", "J\u030C"}, 'ΐ': {"ΐ", "Ι\u0308\u0301", "Ι\u0308\u0301"}, 'ΰ': {"ΰ", "Υ\u0308\u0301", "Υ\u0308\u0301"},
    'և': {"և", "Եւ", "ԵՒ"}, 'ẖ': {"ẖ", "H\u0331", "H\u0331"}, 'ẗ': {"ẗ", "T\u0308", "T\u0308"},
    'ẘ': {"ẘ", "W\u030A", "W\u030A"}, 'ẙ': {"ẙ", "Y\u030A", "Y\u030A"}, 'ẚ': {"ẚ", "Aʾ", "Aʾ"},
    'ὐ': {"ὐ", "Υ\u0313", "Υ\u0313"}, 'ὒ': {"ὒ", "Υ\u0313\u0300", "Υ\u0313\u0300"}, 'ὔ': {"ὔ", "Υ\u0313\u0301", "Υ\u0313\u0301"},
    'ὖ': {"ὖ", "Υ\u0313\u0342", "Υ\u0313\u0342"}, 'ᾀ': {"ᾀ", "ᾈ", "ἈΙ"}, 'ᾁ': {"ᾁ", "ᾉ", "ἉΙ"},
    'ᾂ': {"ᾂ", "ᾊ", "ἊΙ"}, 'ᾃ': {"ᾃ", "ᾋ", "ἋΙ"}, 'ᾄ': {"ᾄ", "ᾌ", "ἌΙ"},
    'ᾅ': {"ᾅ", "ᾍ", "ἍΙ"}, 'ᾆ': {"ᾆ", "ᾎ", "ἎΙ"}, 'ᾇ': {"ᾇ", "ᾏ", "ἏΙ"},
    'ᾈ': {"ᾀ", "ᾈ", "ἈΙ"}, 'ᾉ': {"ᾁ", "ᾉ", "ἉΙ"}, 'ᾊ': {"ᾂ", "ᾊ", "ἊΙ"},
    'ᾋ': {"ᾃ", "ᾋ", "ἋΙ"}, 'ᾌ': {"ᾄ", "ᾌ", "ἌΙ"}, 'ᾍ': {"ᾅ", "ᾍ", "ἍΙ"},
    'ᾎ': {"ᾆ", "ᾎ", "ἎΙ"}, 'ᾏ': {"ᾇ", "ᾏ", "ἏΙ"}, 'ᾐ': {"ᾐ", "ᾘ", "ἨΙ"},
    'ᾑ': {"ᾑ", "ᾙ", "ἩΙ"}, 'ᾒ': {"ᾒ", "ᾚ", "ἪΙ"}, 'ᾓ': {"ᾓ", "ᾛ", "ἫΙ"},
    'ᾔ': {"ᾔ", "ᾜ", "ἬΙ"}, 'ᾕ': {"ᾕ", "ᾝ", "ἭΙ"}, 'ᾖ': {"ᾖ", "ᾞ", "ἮΙ"},
    'ᾗ': {"ᾗ", "ᾟ", "ἯΙ"}, 'ᾘ': {"ᾐ", "ᾘ", "ἨΙ"}, 'ᾙ': {"ᾑ", "ᾙ", "ἩΙ"},
    'ᾚ': {"ᾒ", "ᾚ", "ἪΙ"}, 'ᾛ': {"ᾓ", "ᾛ", "ἫΙ"}, 'ᾜ': {"ᾔ", "ᾜ", "ἬΙ"},
    'ᾝ': {"ᾕ", "ᾝ", "ἭΙ"}, 'ᾞ': {"ᾖ", "ᾞ", "ἮΙ"}, 'ᾟ': {"ᾗ", "ᾟ", "ἯΙ"},
    'ᾠ': {"ᾠ", "ᾨ", "ὨΙ"}, 'ᾡ': {"ᾡ", "ᾩ", "ὩΙ"}, 'ᾢ': {"ᾢ", "ᾪ", "ὪΙ"},
    'ᾣ': {"ᾣ", "ᾫ", "ὫΙ"}, 'ᾤ': {"ᾤ", "ᾬ", "ὬΙ"}, 'ᾥ': {"ᾥ", "ᾭ", "ὭΙ"},
    'ᾦ': {"ᾦ", "ᾮ", "ὮΙ"}, 'ᾧ': {"ᾧ", "ᾯ", "ὯΙ"}, 'ᾨ': {"ᾠ", "ᾨ", "ὨΙ"},
    'ᾩ': {"ᾡ", "ᾩ", "ὩΙ"}, 'ᾪ': {"ᾢ", "ᾪ", "ὪΙ"}, 'ᾫ': {"ᾣ", "ᾫ", "ὫΙ"},
    'ᾬ': {"ᾤ", "ᾬ", "ὬΙ"}, 'ᾭ': {"ᾥ", "ᾭ", "ὭΙ"}, 'ᾮ': {"ᾦ", "ᾮ", "ὮΙ"},
    'ᾯ': {"ᾧ", "ᾯ", "ὯΙ"}, 'ᾲ': {"ᾲ", "Ὰ\u0345", "ᾺΙ"}, 'ᾳ': {"ᾳ", "ᾼ", "ΑΙ"},
    'ᾴ': {"ᾴ", "Ά\u0345", "ΆΙ"}, 'ᾶ': {"ᾶ", "Α\u0342", "Α\u0342"}, 'ᾷ': {"ᾷ", "Α\u0342\u0345", "Α\u0342Ι"},
    'ᾼ': {"ᾳ", "ᾼ", "ΑΙ"}, 'ῂ': {"ῂ", "Ὴ\u0345", "ῊΙ"}, 'ῃ': {"ῃ", "ῌ", "ΗΙ"},
    'ῄ': {"ῄ", "Ή\u0345", "ΉΙ"}, 'ῆ': {"ῆ", "Η\u0342", "Η\u0342"}, 'ῇ': {"ῇ", "Η\u0342\u0345", "Η\u0342Ι"},
    'ῌ': {"ῃ", "ῌ", "ΗΙ"}, 'ῒ': {"ῒ", "Ι\u0308\u0300", "Ι\u0308\u0300"}, 'ΐ': {"ΐ", "Ι\u0308\u0301", "Ι\u0308\u0301"},
    'ῖ': {"ῖ", "Ι\u0342", "Ι\u0342"}, 'ῗ': {"ῗ", "Ι\u0308\u0342", "Ι\u0308\u0342"}, 'ῢ': {"ῢ", "Υ\u0308\u0300", "Υ\u0308\u0300"},
    'ΰ': {"ΰ", "Υ\u0308\u0301", "Υ\u0308\u0301"}, 'ῤ': {"ῤ", "Ρ\u0313", "Ρ\u0313"}, 'ῦ': {"ῦ", "Υ\u0342", "Υ\u0342"},
    'ῧ': {"ῧ", "Υ\u0308\u0342", "Υ\u0308\u0342"}, 'ῲ': {"ῲ", "Ὼ\u0345", "ῺΙ"}, 'ῳ': {"ῳ", "ῼ", "ΩΙ"},
    'ῴ': {"ῴ", "Ώ\u0345", "ΏΙ"}, 'ῶ': {"ῶ", "Ω\u0342", "Ω\u0342"}, 'ῷ': {"ῷ", "Ω\u0342\u0345", "Ω\u0342Ι"},
    'ῼ': {"ῳ", "ῼ", "ΩΙ"}, 'ﬀ': {"ﬀ", "Ff", "FF"}, 'ﬁ': {"ﬁ", "Fi", "FI"},
    'ﬂ': {"ﬂ", "Fl", "FL"}, 'ﬃ': {"ﬃ", "Ffi", "FFI"}, 'ﬄ': {"ﬄ", "Ffl", "FFL"},
    'ﬅ': {"ﬅ", "St", "ST"}, 'ﬆ': {"ﬆ", "St", "ST"}, 'ﬓ': {"ﬓ", "Մն", "ՄՆ"},
    'ﬔ': {"ﬔ", "Մե", "ՄԵ"}, 'ﬕ': {"ﬕ", "Մի", "ՄԻ"}, 'ﬖ': {"ﬖ", "Վն", "ՎՆ"},
    'ﬗ': {"ﬗ", "Մխ", "ՄԽ"},
  }

  // rbCaseMap is MRI's full case mapping: column i of rbSpecialCasing, else f.
  func rbCaseMap(s string, i int, f func(rune) rune) string {
    var b strings.Builder
    b.Grow(len(s))
    for _, r := range s {
      if r >= 'ß' { // the lowest key: ASCII skips the lookup
        if m, ok := rbSpecialCasing[r]; ok {
          b.WriteString(m[i])
          continue
        }
      }
      b.WriteRune(f(r))
    }
    return b.String()
  }

  // rbSub replaces the first n (all if n < 0) occurrences of pat, expanding
  // MRI's backslash sequences in rep: \\0 \\& match, \\` \\' pre/post-match,
  // \\\\ a backslash; \\1-\\9 and \\+ are empty (a String pattern has no groups).
  // ponytail: \\k<name> stays literal; MRI raises IndexError.
  func rbSub(s, pat, rep string, n int) string {
    if !strings.Contains(rep, "\\\\") {
      return strings.Replace(s, pat, rep, n)
    }
    var b strings.Builder
    done, pos := 0, 0
    for ; n != 0; n-- {
      i := strings.Index(s[pos:], pat)
      if i < 0 {
        break
      }
      m, e := pos+i, pos+i+len(pat)
      b.WriteString(s[done:m])
      for j := 0; j < len(rep); j++ {
        if rep[j] != '\\\\' || j+1 == len(rep) {
          b.WriteByte(rep[j])
          continue
        }
        j++
        switch rep[j] {
        case '0', '&':
          b.WriteString(pat)
        case '`':
          b.WriteString(s[:m])
        case '\\'':
          b.WriteString(s[e:])
        case '\\\\':
          b.WriteByte('\\\\')
        case '1', '2', '3', '4', '5', '6', '7', '8', '9', '+':
        default:
          b.WriteString(rep[j-1 : j+1])
        }
      }
      done, pos = e, e
      if pat == "" { // step past a rune so an empty pattern matches between each
        if pos == len(s) {
          break
        }
        _, w := utf8.DecodeRuneInString(s[pos:])
        pos += w
      }
    }
    b.WriteString(s[done:])
    return b.String()
  }
}

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
        *out = append(*out, rbNewStr(self, String(l)))
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
        *out = append(*out, rbNewStr(self, String(f)))
      }
      return out
    }
    parts := strings.Split(string(self), string(*sep))
    for len(parts) > 0 && parts[len(parts)-1] == "" {
      parts = parts[:len(parts)-1]
    }
    for _, p := range parts {
      *out = append(*out, rbNewStr(self, String(p)))
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

  #: (Integer) -> String
  def center(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return rbStrClone(self)
    }
    return String(strings.Repeat(" ", n/2) + string(self) + strings.Repeat(" ", n-n/2))
  }

  #: (Integer) -> String
  def ljust(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return rbStrClone(self)
    }
    return self + String(strings.Repeat(" ", n))
  }

  #: (Integer) -> String
  def rjust(width) = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if n <= 0 {
      return rbStrClone(self)
    }
    return String(strings.Repeat(" ", n)) + self
  }
end
