# prelude/regexp.rb
# rbs_inline: enabled
#
# Regexp on Go's RE2. The transpiler translates literals (every Ruby regexp
# gets `(?m)`: `^`/`$` are line anchors in Ruby) and rejects what RE2 cannot
# match; see internal/compiler/regexp.go. rxRegexp (rxtranslate.go) matches
# Onigmo's `^`, `\b` and multi-character `/i` folds where RE2's differ.

%x{
  // rbRegexpNew compiles a translated pattern; dynamic (interpolated) ones
  // may fail at run time, like Ruby's.
  func rbRegexpNew(pattern, src, opts string) *Regexp {
    re, err := regexp.Compile(rxFold(pattern))
    if err != nil {
      panic(NewRegexpError(Ref(String(err.Error()))))
    }
    return &Regexp{re: rxNew(re), src: src, opts: opts}
  }

  // rbRegexpDyn compiles an interpolated regexp from its Ruby source, which
  // only exists at run time; translateRegexp is the compiler's own
  // (internal/compiler/rxtranslate.go, emitted into every program).
  func rbRegexpDyn(prefix, src, opts string) *Regexp {
    pat, err := translateRegexp(src)
    if err != nil {
      panic(NewRegexpError(Ref(String(err.Error()))))
    }
    return rbRegexpNew(prefix+pat, src, opts)
  }

  // rbRegexpDesc is the source as inspect and to_s show it: a bare / is
  // escaped, as MRI's rb_reg_desc does.
  func rbRegexpDesc(src string) string {
    var b strings.Builder
    for i := 0; i < len(src); i++ {
      switch {
      case src[i] == '\\\\' && i+1 < len(src):
        b.WriteString(src[i : i+2])
        i++
      case src[i] == '/':
        b.WriteString(`\\/`)
      default:
        b.WriteByte(src[i])
      }
    }
    return b.String()
  }

  func rbMatch(r *Regexp, s string) *MatchData {
    loc := r.re.FindStringSubmatchIndex(s)
    if loc == nil {
      return nil
    }
    groups := make([]*String, len(loc)/2)
    for i := range groups {
      if loc[2*i] >= 0 {
        g := String(s[loc[2*i]:loc[2*i+1]])
        groups[i] = &g
      }
    }
    return &MatchData{groups: groups, names: r.re.SubexpNames(), pre: s[:loc[0]], post: s[loc[1]:]}
  }

  // rbSubject is the text a Regexp matches: a String, or a Symbol's name.
  // nil matches nothing (ok is false); anything else is MRI's TypeError, and
  // a String that is not valid UTF-8 is MRI's ArgumentError.
  func rbSubject(v any) (string, bool) {
    switch s := v.(type) {
    case nil:
      return "", false
    case String:
      if !utf8.ValidString(string(s)) {
        panic(NewArgumentError(Ref(String("invalid byte sequence in UTF-8"))))
      }
      return string(s), true
    case Symbol:
      return string(s), true
    }
    panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into String"))))
  }
}

# @go_type struct { re *rxRegexp; src string; opts string }
class Regexp < Object
  # A subject is a String for typed callers; an untyped one may also be a
  # Symbol (its name is matched) or nil (no match), as in MRI.
  #: (String | untyped) -> bool
  def match?(s) = %x{
    str, ok := rbSubject(s)
    return Boolean(ok && self.re.MatchString(str))
  }

  #: (String | untyped) -> MatchData?
  def match(s) = %x{
    str, ok := rbSubject(s)
    if !ok {
      return nil
    }
    m := rbMatch(self, str)
    if m == nil {
      return nil
    }
    return &m
  }

  # The character (not byte) index of the first match.
  #: (String | untyped) -> Integer?
  def =~(s) = %x{
    str, ok := rbSubject(s)
    if !ok {
      return nil
    }
    loc := self.re.FindStringIndex(str)
    if loc == nil {
      return nil
    }
    return Ref(Integer(utf8.RuneCountInString(str[:loc[0]])))
  }

  # `case x when /re/` calls this; unlike match?, other types are false.
  #: (untyped) -> bool
  def ===(other) = %x{
    switch other.(type) {
    case String, Symbol:
      s, _ := rbSubject(other)
      return Boolean(self.re.MatchString(s))
    }
    return false
  }

  #: () -> String
  def source = %x{ String(self.src) }

  #: () -> String
  def inspect = %x{ String("/" + rbRegexpDesc(self.src) + "/" + self.opts) }

  #: () -> String
  def to_s = %x{
    var on, off strings.Builder
    for _, f := range "mix" {
      if strings.ContainsRune(self.opts, f) {
        on.WriteRune(f)
      } else {
        off.WriteRune(f)
      }
    }
    flags := on.String()
    if off.Len() > 0 {
      flags += "-" + off.String()
    }
    return String("(?" + flags + ":" + rbRegexpDesc(self.src) + ")")
  }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Regexp)
    return Boolean(ok && o.src == self.src && o.opts == self.opts)
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{ Integer(maphash.String(rbHashSeed, self.src+"/"+self.opts)) }
end

# names are the groups' names ("" when unnamed), as Go's SubexpNames.
# @go_type struct { groups []*String; names []string; pre string; post string }
class MatchData < Object
  #: (Integer) -> String?
  def [](i) = %x{
    if i < 0 {
      // MRI (rb_reg_nth_match): a negative index never reaches group 0.
      i += Integer(len(self.groups))
      if i <= 0 {
        return nil
      }
    }
    if int(i) >= len(self.groups) {
      return nil
    }
    return self.groups[i]
  }

  #: () -> Array[String?]
  def captures = %x{
    out := &Array[*String]{}
    *out = append(*out, self.groups[1:]...)
    return out
  }

  #: () -> String
  def pre_match = %x{ String(self.pre) }

  #: () -> String
  def post_match = %x{ String(self.post) }

  #: () -> String
  def to_s = self[0].to_s

  #: () -> String
  def inspect = %x{
    var b strings.Builder
    b.WriteString("#<MatchData ")
    for i, g := range self.groups {
      switch {
      case i == 0:
      case self.names[i] != "":
        b.WriteString(" " + self.names[i] + ":")
      default:
        fmt.Fprintf(&b, " %d:", i)
      }
      if g == nil {
        b.WriteString("nil")
      } else {
        b.WriteString(string(rbStringInspect(string(*g))))
      }
    }
    b.WriteString(">")
    return String(b.String())
  }
end

class String
  #: (Regexp) -> Integer?
  def =~(re) = re =~ self

  #: (Regexp) -> bool
  def !~(re) = !(re =~ self)

  #: (Regexp) -> MatchData?
  def match(re) = re.match(self)

  #: (Regexp) -> bool
  def match?(re) = re.match?(self)
end

class RegexpError < StandardError; end
