# prelude/strscan.rb
# rbs_inline: enabled
#
# StringScanner over a byte position, as MRI's. A pattern is matched
# against the rest of the string, so ^ and \A match at the scan pointer
# (MRI's default, fixed_anchor: false). Always defined.

# last is the previous pointer (for unscan); mbeg/mend bound the last match
# (absolute byte offsets); groups is nil after a failed match.
# @go_type struct { str string; pos int; last int; mbeg int; mend int; groups []*String; names []string }
class StringScanner < Object
  #: (String) -> StringScanner
  def self.new(s) = %x{ return &StringScanner{str: string(s)} }

  #: () -> String
  def string = %x{ String(self.str) }

  #: (String) -> String
  def string=(s)
    %x{
    self.str, self.pos, self.groups = string(s), 0, nil
    return s}
  end

  #: () -> Integer
  def pos = %x{ Integer(self.pos) }

  #: () -> Integer
  def pointer = pos

  #: (Integer) -> Integer
  def pos=(n)
    %x{
    if n < 0 {
      n += Integer(len(self.str))
    }
    if n < 0 || int(n) > len(self.str) {
      panic(NewRangeError(Ref(String("index out of range"))))
    }
    self.pos = int(n)
    return n}
  end

  #: () -> Integer
  def charpos = %x{ Integer(utf8.RuneCountInString(self.str[:self.pos])) }

  #: () -> bool
  def eos? = %x{ Boolean(self.pos >= len(self.str)) }

  #: () -> String
  def rest = %x{ String(self.str[self.pos:]) }

  #: () -> Integer
  def rest_size = %x{ Integer(len(self.str) - self.pos) }

  #: () -> bool
  def beginning_of_line? = %x{ Boolean(self.pos == 0 || self.str[self.pos-1] == '\\n') }

  #: () -> bool
  def bol? = beginning_of_line?

  #: (Regexp) -> String?
  def scan(re) = %x{ return self.do(re, true, true, true) }

  #: (Regexp) -> Integer?
  def skip(re) = %x{ return rbScanLen(self.do(re, true, true, false), self) }

  #: (Regexp) -> Integer?
  def match?(re) = %x{ return rbScanLen(self.do(re, true, false, false), self) }

  #: (Regexp) -> String?
  def check(re) = %x{ return self.do(re, true, false, true) }

  #: (Regexp) -> String?
  def scan_until(re) = %x{ return self.do(re, false, true, true) }

  #: (Regexp) -> Integer?
  def skip_until(re) = %x{ return rbScanLen(self.do(re, false, true, false), self) }

  #: (Regexp) -> String?
  def check_until(re) = %x{ return self.do(re, false, false, true) }

  #: (Regexp) -> Integer?
  def exist?(re) = %x{ return rbScanLen(self.do(re, false, false, false), self) }

  #: () -> String?
  def getch = %x{
    if self.pos >= len(self.str) {
      self.groups = nil
      return nil
    }
    _, n := utf8.DecodeRuneInString(self.str[self.pos:])
    return self.advance(self.pos, self.pos+n)
  }

  #: () -> String?
  def get_byte = %x{
    if self.pos >= len(self.str) {
      self.groups = nil
      return nil
    }
    return self.advance(self.pos, self.pos+1)
  }

  #: (Integer) -> String
  def peek(n) = %x{ String(self.str[self.pos:min(len(self.str), self.pos+int(n))]) }

  #: () -> StringScanner
  def unscan = %x{
    if self.groups == nil {
      panic(NewStringScanner_Error(Ref(String("unscan error: not scanned yet"))))
    }
    self.pos, self.groups = self.last, nil
    return self
  }

  #: () -> StringScanner
  def reset = %x{
    self.pos, self.groups = 0, nil
    return self
  }

  #: () -> StringScanner
  def terminate = %x{
    self.pos, self.groups = len(self.str), nil
    return self
  }

  #: () -> bool
  def matched? = %x{ Boolean(self.groups != nil) }

  #: () -> String?
  def matched = self[0]

  #: () -> Integer?
  def matched_size = %x{
    if self.groups == nil {
      return nil
    }
    return Ref(Integer(self.mend - self.mbeg))
  }

  #: () -> String?
  def pre_match = %x{
    if self.groups == nil {
      return nil
    }
    return Ref(String(self.str[:self.mbeg]))
  }

  #: () -> String?
  def post_match = %x{
    if self.groups == nil {
      return nil
    }
    return Ref(String(self.str[self.mend:]))
  }

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

  #: () -> Array[String?]?
  def captures = %x{
    if self.groups == nil {
      return nil
    }
    out := &Array[*String]{}
    out.s = append(out.s, self.groups[1:]...)
    return &out
  }

  #: () -> Hash[String, String?]
  def named_captures = %x{
    out := NewHash[String, *String]()
    if self.groups == nil {
      return out
    }
    for i, name := range self.names {
      if name != "" {
        Hash_Op_idxSet(out, String(name), self.groups[i])
      }
    }
    return out
  }

  #: (*Integer) -> Array[String?]
  def values_at(*indices) = indices.map { |i| self[i] }

  #: (String) -> self
  def <<(s) = %x{
    self.str += string(s)
    return self
  }

  #: (String) -> self
  def concat(s) = self << s

  #: (Regexp, bool, bool) -> untyped
  def scan_full(re, advance_pointer_p, return_string_p) = %x{
    m := self.do(re, true, bool(advance_pointer_p), true)
    if !bool(return_string_p) {
      return Opt(rbScanLen(m, self))
    }
    return Opt(m)
  }

  #: (Regexp, bool, bool) -> untyped
  def search_full(re, advance_pointer_p, return_string_p) = %x{
    m := self.do(re, false, bool(advance_pointer_p), true)
    if !bool(return_string_p) {
      return Opt(rbScanLen(m, self))
    }
    return Opt(m)
  }

  #: () -> String
  def inspect = %x{
    if self.pos >= len(self.str) {
      return "#<StringScanner fin>"
    }
    s := fmt.Sprintf("#<StringScanner %d/%d", self.pos, len(self.str))
    if self.pos > 0 {
      before := self.str[max(0, self.pos-5):self.pos]
      if self.pos > 5 {
        before = "..." + before
      }
      s += " " + rbByteInspect(before)
    }
    after := self.str[self.pos:min(len(self.str), self.pos+5)]
    if len(self.str)-self.pos > 5 {
      after += "..."
    }
    return String(s + " @ " + rbByteInspect(after) + ">")
  }

  class Error < StandardError; end
end
