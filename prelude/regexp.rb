# prelude/regexp.rb
# rbs_inline: enabled
#
# Regexp on Go's RE2. The transpiler translates literals (every Ruby regexp
# gets `(?m)`: `^`/`$` are line anchors in Ruby) and rejects what RE2 cannot
# match; see internal/compiler/regexp.go. rxRegexp (rxtranslate.go) matches
# Onigmo's `^`, `\b` and multi-character `/i` folds where RE2's differ.

# @go_type struct { re *rxRegexp; src string; opts string; anch *atomic.Pointer[rxRegexp] }
class Regexp < Object
  # Regexp.last_match(n): the compiler passes the caller's $~ (decision 232).
  #: (MatchData?, untyped) -> String?
  def self.__last_match_at(m, n) = m ? m[n] : nil

  IGNORECASE = 1 #: Integer
  EXTENDED = 2 #: Integer
  MULTILINE = 4 #: Integer
  FIXEDENCODING = 16 #: Integer
  NOENCODING = 32 #: Integer

  # A regexp2 match (lookaround, backreference) that ran past 10 seconds; RE2 matches are linear and never raise it (decision 242).
  class TimeoutError < RegexpError; end

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

  # US-ASCII for an ASCII-only source, else UTF-8 (or with //u), as MRI derives it (decision 191).
  # ponytail: //n with a \xNN escape is BINARY in MRI and US-ASCII here.
  #: () -> Encoding
  def encoding = %x{
    if strings.ContainsRune(self.opts, 'u') || strings.Contains(self.src, `\\u`) || strings.IndexFunc(self.src, func(r rune) bool { return r >= 0x80 }) >= 0 {
      return Encoding_UTF_8
    }
    return Encoding_US_ASCII
  }
  # A subject is a String for typed callers; an untyped one may also be a
  # Symbol (its name is matched) or nil (no match), as in MRI.
  #: (String | untyped) -> bool
  def match?(s) = %x{
    str, ok := rbSubject(s)
    return Boolean(ok && rbMatches(self, str))
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
    loc := rbFindLoc(self, str)
    if loc == nil {
      return nil
    }
    return Ref(Integer(rbStrLen(str[:loc[0]])))
  }

  # Ruby's rb_reg_quote: metacharacters, space and the whitespace escapes.
  # A Symbol's name too, as MRI's (Homebrew's `Version.formula_optionally_versioned_regex(:glibc)`).
  #: (String | Symbol) -> String
  def self.escape(str) = %x{ return String(rbRegexpEscape(string(rbToS(any(str))))) }

  # `case x when /re/` calls this; unlike match?, other types are false.
  #: (untyped) -> bool
  def ===(other) = %x{
    switch other.(type) {
    case String, Symbol:
      s, _ := rbSubject(other)
      return Boolean(rbMatches(self, s))
    }
    return false
  }

  #: () -> String
  def source = %x{ String(self.src) }

  # A binary String subject (decision 352): each byte is a character, and =~ is a byte index.
  #: (String) -> Integer?
  def __bin_eqtilde(s) = %x{
    loc := rbFindLocBin(self, string(s))
    if loc == nil {
      return nil
    }
    return Ref(Integer(loc[0]))
  }

  #: (String) -> MatchData?
  def __bin_match(s) = %x{
    loc := rbFindLocBin(self, string(s))
    if loc == nil {
      return nil
    }
    m := rbMatchAt(self, string(s), loc)
    return &m
  }

  #: (String) -> bool
  def __bin_match_q(s) = %x{ return Boolean(rbFindLocBin(self, string(s)) != nil) }

  #: (String) -> bool
  def __bin_eqq(s) = %x{ return Boolean(rbFindLocBin(self, string(s)) != nil) }

  # Whether the pattern has capture groups: String#scan then gives each match's groups (decision 331).
  #: () -> bool
  def __groups? = %x{ Boolean(self.re.NumSubexp() > 0) }

  #: (String | Symbol) -> String
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

  # The whole match, then the groups.
  #: () -> Array[String?]
  def to_a = %x{
    out := &Array[*String]{}
    out.s = append(out.s, self.groups...)
    return out
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

  #: (*(Integer | String | Symbol)) -> Array[String?]
  def values_at(*idx)
    idx.map do |i|
      case i
      when Integer then self[i]
      when Symbol then __idx_symbol(i)
      else __idx_string(i)
      end
    end
  end

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

  # A named group's index (the last of a repeated name, as MRI), for the name twins of begin/end/offset/byteoffset.
  #: (String | Symbol) -> Integer
  def __group(name) = %x{
    var s string
    switch v := name.(type) {
    case String:
      s = string(v)
    case Symbol:
      s = string(v)
    }
    for i := len(self.names) - 1; i > 0; i-- {
      if self.names[i] == s {
        return Integer(i)
      }
    }
    panic(NewIndexError(Ref(String("undefined group name reference: " + s))))
  }

  #: (String) -> Integer?
  def __begin_string(name) = self.begin(__group(name))

  #: (Symbol) -> Integer?
  def __begin_symbol(name) = self.begin(__group(name))

  #: (String) -> Integer?
  def __end_string(name) = self.end(__group(name))

  #: (Symbol) -> Integer?
  def __end_symbol(name) = self.end(__group(name))

  #: (String) -> [Integer?, Integer?]
  def __offset_string(name) = offset(__group(name))

  #: (Symbol) -> [Integer?, Integer?]
  def __offset_symbol(name) = offset(__group(name))

  #: (String) -> [Integer?, Integer?]
  def __byteoffset_string(name) = byteoffset(__group(name))

  #: (Symbol) -> [Integer?, Integer?]
  def __byteoffset_symbol(name) = byteoffset(__group(name))

  # Ruby 3.4's bytebegin/byteend: a group's byte offsets (byteoffset's halves).
  #: (Integer) -> Integer?
  def bytebegin(n) = byteoffset(n)[0]

  #: (Integer) -> Integer?
  def byteend(n) = byteoffset(n)[1]

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

  # The match or a group of it, by index or by name (Homebrew's `ref[REGEX, :name]`, decision 255).
  #: (Regexp, ?untyped) -> String?
  def __idx_regexp(re, n = 0)
    m = re.match(self)
    return nil unless m

    case n
    when Integer then m[n]
    when Symbol then m[n]
    when String then m[n]
    else raise TypeError, __idx_conv_message(n)
    end
  end

  # MRI's message for a bad group argument, from Go (an interpolated `n.class` cost every program with a dynamic `[]`
  # a thousand functions).
  #: (untyped) -> String
  def __idx_conv_message(n) = %x{ return String("no implicit conversion of " + rbClassName(n) + " into Integer") }

  #: (Regexp) -> bool
  def !~(re) = !(re =~ self)

  #: (Regexp) -> MatchData?
  def match(re) = re.match(self)

  #: (Regexp) -> bool
  def match?(re) = re.match?(self)

  # A binary String's matches read its bytes (decision 352).
  #: (Regexp) -> Integer?
  def __bin_eqtilde(re) = re.__bin_eqtilde(self)

  #: (Regexp) -> MatchData?
  def __bin_match(re) = re.__bin_match(self)

  #: (Regexp) -> bool
  def __bin_match_q(re) = re.__bin_match_q(self)

  # A match that starts at 0 (Go's leftmost match starts at 0 whenever one can). ponytail: $~ is not set; set it
  # when a caller reads it after start_with?.
  #: (Regexp) -> bool
  def __start_with_q_regexp(re) = %x{
    locs := rbFindAll(re, string(self), 1)
    return Boolean(len(locs) > 0 && locs[0][0] == 0)
  }

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
    for _, loc := range rbFindAll(re, string(self), -1) {
      out.s = append(out.s, self[loc[0]:loc[1]])
    }
    return out
  }

  # scan where the pattern is known only at run time (a gem's untyped receiver, decision 331): each match, or each
  # match's groups when the pattern has them, as MRI decides.
  #: (Regexp) -> untyped
  def __scan_any(re) = re.__groups? ? __scan_groups(re) : scan(re)

  # The block form: the same values, yielded; self after.
  #: (Regexp) { (untyped) -> untyped } -> String
  def __scan_block(re)
    if re.__groups?
      __scan_groups(re).each { |g| yield g }
    else
      scan(re).each { |m| yield m }
    end
    self
  end

  #: (Regexp) -> Array[Array[String?]]
  def __scan_groups(re) = %x{
    out := &Array[*Array[*String]]{}
    for _, loc := range rbFindAll(re, string(self), -1) {
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

  # MRI's rb_str_split_m: captured groups are kept, an empty match at the field start does not split, and a pattern
  # matching the empty string splits between characters. Its searches are the matches of scan, so they come from FindAll.
  #: (Regexp, ?Integer) -> Array[String]
  def __split_regexp(re, limit = 0) = %x{
    s := string(self)
    out := &Array[String]{}
    if s == "" {
      return out
    }
    lim := int(limit)
    if lim == 1 {
      out.s = append(out.s, String(s))
      return out
    }
    beg, start, fields, lastNull := 0, 0, 1, false
    field := func(end int, loc []int) bool { // pushes s[beg:end] and the groups; reports the limit reached
      out.s = append(out.s, String(s[beg:end]))
      for i := 2; i < len(loc); i += 2 {
        if loc[i] >= 0 {
          out.s = append(out.s, String(s[loc[i]:loc[i+1]]))
        }
      }
      fields++
      return lim > 0 && lim <= fields
    }
    for _, loc := range rbFindAll(re, s, -1) {
      _, w := utf8.DecodeRuneInString(s[loc[1]:])
      if loc[0] == start && loc[1] == start { // empty at the search start: skip a character, then split before the next match
        if !lastNull {
          start += max(w, 1)
          lastNull = true
          continue
        }
        done := field(start, loc)
        beg = start
        if done {
          break
        }
      } else {
        done := field(loc[0], loc)
        beg, start = loc[1], loc[1]
        if done {
          break
        }
        if loc[0] != loc[1] {
          lastNull = false
          continue
        }
      }
      start += max(w, 1)
      lastNull = true
    }
    if lim != 0 || len(s) > beg {
      out.s = append(out.s, String(s[beg:]))
    }
    for lim == 0 && len(out.s) > 0 && out.s[len(out.s)-1] == "" {
      out.s = out.s[:len(out.s)-1]
    }
    return out
  }
end

class RegexpError < StandardError; end
