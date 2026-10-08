# prelude/regexp.rb
# rbs_inline: enabled
#
# Regexp on Go's RE2. The transpiler translates literals (every Ruby regexp
# gets `(?m)`: `^`/`$` are line anchors in Ruby) and rejects what RE2 cannot
# match; see internal/compiler/regexp.go. rxRegexp (rxtranslate.go) matches
# Onigmo's `^`, `\b` and multi-character `/i` folds where RE2's differ.

# @go_type struct { re *rxRegexp; src string; opts string }
class Regexp < Object
  IGNORECASE = 1 #: Integer
  EXTENDED = 2 #: Integer
  MULTILINE = 4 #: Integer
  FIXEDENCODING = 16 #: Integer
  NOENCODING = 32 #: Integer

  # A Regexp from a String at run time, translated from Ruby's syntax as an
  # interpolated literal is (decision 115). options: an Integer of
  # IGNORECASE | MULTILINE, a String of "m"/"i" flags, true for /i, or nil.
  #: (untyped, ?untyped) -> Regexp
  def self.new(pattern, options = nil) = %x{ return rbRegexpFromValue(pattern, options) }

  #: (untyped, ?untyped) -> Regexp
  def self.compile(pattern, options = nil) = %x{ return rbRegexpFromValue(pattern, options) }

  #: () -> Integer
  def options = %x{
    n := 0
    for _, f := range self.opts {
      n |= map[rune]int{'i': 1, 'x': 2, 'm': 4}[f]
    }
    return Integer(n)
  }

  #: () -> bool
  def casefold? = %x{ return Boolean(strings.ContainsRune(self.opts, 'i')) }
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

  # Ruby's rb_reg_quote: metacharacters, space and the whitespace escapes.
  #: (String) -> String
  def self.escape(str) = %x{ return String(rbRegexpEscape(string(str))) }

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

  #: (String) -> String
  def self.quote(str) = escape(str)

  # A Regexp matching any of pats: a String matches itself, a Regexp keeps its flags (its to_s).
  #: (*untyped) -> Regexp
  def self.union(*pats)
    pats = pats[0] if pats.size == 1 && pats[0].is_a?(Array)
    # ponytail: MRI's empty union is /(?!)/, which RE2 cannot compile; this never-matching class inspects differently
    return Regexp.new("[^\\s\\S]") if pats.empty?
    return pats[0] if pats.size == 1 && pats[0].is_a?(Regexp)
    parts = pats.map { |x| x.is_a?(Regexp) ? x.to_s : Regexp.escape(x.to_s) }
    Regexp.new(parts.join("|"))
  end

  #: () -> Array[String]
  def names = %x{ return rbUniqNames(self.re.SubexpNames()) }

  #: () -> Hash[String, Array[Integer]]
  def named_captures
    out = {} #: Hash[String, Array[Integer]]
    __group_names.each_with_index do |n, i|
      next if n.empty?
      (out[n] ||= []) << i
    end
    out
  end

  #: () -> Array[String]
  def __group_names = %x{
    out := &Array[String]{}
    for _, n := range self.re.SubexpNames() {
      out.s = append(out.s, String(n))
    }
    return out
  }

  # Every Regexp is a literal, and literals are frozen.
  #: () -> bool
  def frozen? = true

  #: () -> self
  def freeze = self

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

# names are the groups' names ("" when unnamed), as Go's SubexpNames; loc holds byte offsets into subj.
# @go_type struct { groups []*String; names []string; pre string; post string; subj string; loc []int }
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
    out.s = append(out.s, self.groups[1:]...)
    return out
  }

  #: (String) -> String?
  def __idx_string(name) = %x{ return rbMatchNamed(self, string(name)) }

  #: (Symbol) -> String?
  def __idx_symbol(name) = %x{ return rbMatchNamed(self, string(name)) }

  #: () -> Integer
  def size = %x{ Integer(len(self.groups)) }

  #: () -> Integer
  def length = size

  #: () -> String
  def string = %x{ String(self.subj) }

  #: () -> Array[String]
  def names = %x{ return rbUniqNames(self.names) }

  #: () -> Hash[String, String?]
  def named_captures
    out = {} #: Hash[String, String?]
    names.each { |n| out[n] = self[n] }
    out
  end

  #: (*Integer) -> Array[String?]
  def values_at(*idx) = idx.map { |i| self[i] }

  #: (Integer) -> String?
  def match(n)
    self.begin(n)
    self[n]
  end

  #: (Integer) -> Integer?
  def match_length(n) = match(n)&.size

  #: (Integer) -> Integer?
  def begin(n) = %x{ return rbMatchOffset(self, n, 0) }

  #: (Integer) -> Integer?
  def end(n) = %x{ return rbMatchOffset(self, n, 1) }

  #: (Integer) -> [Integer?, Integer?]
  def offset(n) = [self.begin(n), self.end(n)]

  #: (Integer) -> [Integer?, Integer?]
  def byteoffset(n) = %x{
    if n < 0 || int(n) >= len(self.groups) {
      panic(NewIndexError(Ref(String(fmt.Sprintf("index %d out of matches", n)))))
    }
    if self.loc[2*n] < 0 {
      return Tuple2[*Integer, *Integer]{}
    }
    return Tuple2[*Integer, *Integer]{Ref(Integer(self.loc[2*n])), Ref(Integer(self.loc[2*n+1]))}
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

  #: (Regexp) -> Integer?
  def __index_regexp(re) = re =~ self

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
      out.s = append(out.s, self[loc[0]:loc[1]])
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
          row.s = append(row.s, nil)
        } else {
          row.s = append(row.s, Ref(self[loc[i]:loc[i+1]]))
        }
      }
      out.s = append(out.s, row)
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
        out.s = append(out.s, String(s[start:start+w]))
        start += w
        continue
      }
      out.s = append(out.s, String(s[start:loc[0]]))
      for i := 2; i < len(loc); i += 2 {
        if loc[i] >= 0 {
          out.s = append(out.s, String(s[loc[i]:loc[i+1]]))
        }
      }
      start = loc[1]
    }
    out.s = append(out.s, String(s[start:]))
    for len(out.s) > 0 && out.s[len(out.s)-1] == "" {
      out.s = out.s[:len(out.s)-1]
    }
    return out
  }
end

class RegexpError < StandardError; end
