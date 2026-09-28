# prelude/regexp.rb
# rbs_inline: enabled
#
# Regexp on Go's RE2. The transpiler translates literals (every Ruby regexp
# gets `(?m)`: `^`/`$` are line anchors in Ruby) and rejects what RE2 cannot
# match; see internal/compiler/regexp.go. rxRegexp (rxtranslate.go) matches
# Onigmo's `^`, `\b` and multi-character `/i` folds where RE2's differ.

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

  # Every Regexp is a literal, and literals are frozen.
  #: () -> bool
  def frozen? = true

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

  # Replacement strings expand \0 \& \1-\9 \k<name> \` \' and \\.
  #: (Regexp, String) -> String
  def __sub_regexp(re, to) = %x{ String(rbReSub(re, string(self), 1, rbReRepl(string(to)))) }

  #: (Regexp, String) -> String
  def __gsub_regexp(re, to) = %x{ String(rbReSub(re, string(self), -1, rbReRepl(string(to)))) }

  # The block gets each match; its value's to_s replaces it.
  #: (untyped) { (String) -> untyped } -> String
  def __sub_block(pat) = %x{ String(rbReSub(rbPattern(pat), string(self), 1, rbReBlock(blk))) }

  #: (untyped) { (String) -> untyped } -> String
  def __gsub_block(pat) = %x{ String(rbReSub(rbPattern(pat), string(self), -1, rbReBlock(blk))) }

  # A pattern with groups yields each match's groups instead: that is
  # __scan_groups, which the compiler picks for a literal with groups.
  #: (Regexp) -> Array[String]
  def scan(re) = %x{
    if re.re.NumSubexp() > 0 {
      panic(NewNotImplementedError(Ref(String("rb2go: String#scan with capture groups needs a Regexp literal"))))
    }
    out := &Array[String]{}
    for _, loc := range re.re.FindAllStringIndex(string(self), -1) {
      *out = append(*out, self[loc[0]:loc[1]])
    }
    return out
  }

  #: (Regexp) -> Array[Array[String?]]
  def __scan_groups(re) = %x{
    out := &Array[*Array[*String]]{}
    for _, loc := range re.re.FindAllStringSubmatchIndex(string(self), -1) {
      row := &Array[*String]{}
      for i := 2; i < len(loc); i += 2 {
        if loc[i] < 0 {
          *row = append(*row, nil)
        } else {
          *row = append(*row, Ref(self[loc[i]:loc[i+1]]))
        }
      }
      *out = append(*out, row)
    }
    return out
  }

  # MRI's: captured groups are kept, trailing empty fields dropped, and a
  # pattern matching the empty string splits between characters.
  #: (Regexp) -> Array[String]
  def __split_regexp(re) = %x{
    s := string(self)
    out := &Array[String]{}
    start := 0
    for _, loc := range re.re.FindAllStringSubmatchIndex(s, -1) {
      if loc[1] == 0 || loc[0] == len(s) && loc[0] == loc[1] {
        continue
      }
      if loc[1] == start { // an empty match (loc[0] >= start)
        // right at the field start: split off one character
        _, w := utf8.DecodeRuneInString(s[start:])
        if start+w > len(s) {
          continue
        }
        *out = append(*out, String(s[start:start+w]))
        start += w
        continue
      }
      *out = append(*out, String(s[start:loc[0]]))
      for i := 2; i < len(loc); i += 2 {
        if loc[i] >= 0 {
          *out = append(*out, String(s[loc[i]:loc[i+1]]))
        }
      }
      start = loc[1]
    }
    *out = append(*out, String(s[start:]))
    for len(*out) > 0 && (*out)[len(*out)-1] == "" {
      *out = (*out)[:len(*out)-1]
    }
    return out
  }
end

class RegexpError < StandardError; end
