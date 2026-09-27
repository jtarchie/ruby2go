# prelude/regexp.rb
# rbs_inline: enabled
#
# Regexp on Go's RE2. The transpiler translates literals (every Ruby regexp
# gets `(?m)`: `^`/`$` are line anchors in Ruby) and rejects what RE2 cannot
# match; see internal/compiler/regexp.go.

%x{
  // rbRegexpNew compiles a translated pattern; dynamic (interpolated) ones
  // may fail at run time, like Ruby's.
  func rbRegexpNew(pattern, src, opts string) *Regexp {
    re, err := regexp.Compile(pattern)
    if err != nil {
      panic(NewRegexpError(Ref(String(err.Error()))))
    }
    return &Regexp{re: re, src: src, opts: opts}
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
    return &MatchData{groups: groups, pre: s[:loc[0]], post: s[loc[1]:]}
  }
}

# @go_type struct { re *regexp.Regexp; src string; opts string }
class Regexp < Object
  #: (String) -> bool
  def match?(s) = %x{ Boolean(self.re.MatchString(string(s))) }

  #: (String) -> MatchData?
  def match(s) = %x{
    m := rbMatch(self, string(s))
    if m == nil {
      return nil
    }
    return &m
  }

  # The character (not byte) index of the first match.
  #: (String) -> Integer?
  def =~(s) = %x{
    loc := self.re.FindStringIndex(string(s))
    if loc == nil {
      return nil
    }
    return Ref(Integer(utf8.RuneCountInString(string(s)[:loc[0]])))
  }

  # `case str when /re/` calls this.
  #: (untyped) -> bool
  def ===(other) = %x{
    s, ok := other.(String)
    return Boolean(ok && self.re.MatchString(string(s)))
  }

  #: () -> String
  def source = %x{ String(self.src) }

  # Every Regexp is a literal, and literals are frozen.
  #: () -> bool
  def frozen? = true

  #: () -> String
  def inspect = %x{ String("/" + self.src + "/" + self.opts) }

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
    return String("(?" + flags + ":" + self.src + ")")
  }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Regexp)
    return Boolean(ok && o.src == self.src && o.opts == self.opts)
  }
end

# @go_type struct { groups []*String; pre string; post string }
class MatchData < Object
  #: (Integer) -> String?
  def [](i) = %x{
    if i < 0 {
      i += Integer(len(self.groups))
    }
    if i < 0 || int(i) >= len(self.groups) {
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
      if i > 0 {
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
