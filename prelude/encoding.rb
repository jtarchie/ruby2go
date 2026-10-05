# rbs_inline: enabled

# Encoding without a tag on String (decision 136): every String is UTF-8 bytes, so `encoding` is derived and only encode, Integer#chr and IO convert.

# @go_type struct { name string; names []string; ascii, dummy bool }
class Encoding < Object
  #: (String, String, bool, bool) -> Encoding
  def self.__new(name, names, ascii, dummy) = %x{ return &Encoding{name: string(name), names: strings.Split(string(names), ","), ascii: bool(ascii), dummy: bool(dummy)} }

  #: () -> String
  def name = %x{ String(self.name) }

  #: () -> String
  def to_s = name

  #: () -> Array[String]
  def names = %x{
    a := make(Array[String], len(self.names))
    for i, n := range self.names {
      a[i] = String(n)
    }
    return &a
  }

  #: () -> String
  def inspect = %x{
    switch {
    case self.name == "ASCII-8BIT":
      return "#<Encoding:BINARY (ASCII-8BIT)>"
    case self.dummy:
      return String("#<Encoding:" + self.name + " (dummy)>")
    }
    return String("#<Encoding:" + self.name + ">")
  }

  #: () -> bool
  def ascii_compatible? = %x{ Boolean(self.ascii) }

  #: () -> bool
  def dummy? = %x{ Boolean(self.dummy) }

  ASCII_8BIT = __new("ASCII-8BIT", "ASCII-8BIT,BINARY", true, false) #: Encoding
  BINARY = ASCII_8BIT #: Encoding
  UTF_8 = __new("UTF-8", "UTF-8,CP65001,locale,external,filesystem", true, false) #: Encoding
  CP65001 = UTF_8 #: Encoding
  US_ASCII = __new("US-ASCII", "US-ASCII,ASCII,ANSI_X3.4-1968,646", true, false) #: Encoding
  ASCII = US_ASCII #: Encoding
  ANSI_X3_4_1968 = US_ASCII #: Encoding
  UTF_16BE = __new("UTF-16BE", "UTF-16BE,UCS-2BE", false, false) #: Encoding
  UCS_2BE = UTF_16BE #: Encoding
  UTF_16LE = __new("UTF-16LE", "UTF-16LE", false, false) #: Encoding
  UTF_32BE = __new("UTF-32BE", "UTF-32BE,UCS-4BE", false, false) #: Encoding
  UCS_4BE = UTF_32BE #: Encoding
  UTF_32LE = __new("UTF-32LE", "UTF-32LE,UCS-4LE", false, false) #: Encoding
  UCS_4LE = UTF_32LE #: Encoding
  UTF_16 = __new("UTF-16", "UTF-16", false, true) #: Encoding
  UTF_32 = __new("UTF-32", "UTF-32", false, true) #: Encoding
  ISO_8859_1 = __new("ISO-8859-1", "ISO-8859-1,ISO8859-1", true, false) #: Encoding
  ISO8859_1 = ISO_8859_1 #: Encoding

  # MRI's list order, which puts ISO-8859-1 after encodings rb2go lacks.
  LIST__ = [ASCII_8BIT, UTF_8, US_ASCII, UTF_16BE, UTF_16LE, UTF_32BE, UTF_32LE, UTF_16, UTF_32, ISO_8859_1] #: Array[Encoding]

  #: () -> Array[Encoding]
  def self.list = LIST__.dup

  #: () -> Array[String]
  def self.name_list = LIST__.map(&:name) + %w[BINARY ISO8859-1 ASCII ANSI_X3.4-1968 646 CP65001 UCS-2BE UCS-4BE UCS-4LE locale external filesystem]

  #: () -> Hash[String, String]
  def self.aliases
    h = {} #: Hash[String, String]
    LIST__.each { |e| e.names.drop(1).each { |a| h[a] = e.name } }
    h
  end

  # "internal" with no default_internal is nil in MRI; Encoding? would make every find's result optional, so it raises here.
  #: (untyped) -> Encoding
  def self.find(name)
    n = __name_arg(name)
    d = n.casecmp?("internal") ? default_internal : nil
    return d if d
    return default_external if n.casecmp?("external")

    found = LIST__.find { |e| e.names.any? { |x| x.casecmp?(n) } }
    raise ArgumentError, "unknown encoding name - #{n}" unless found

    found
  end

  #: (untyped) -> String
  def self.__name_arg(x) = %x{ return String(rbEncNameArg(x)) }

  #: () -> Encoding
  def self.default_external = %x{
    if rbEncDefaultExternal == nil {
      return Encoding_UTF_8
    }
    return rbEncDefaultExternal
  }

  # Stored for default_external to answer; IO keeps reading and writing UTF-8 (decision 136).
  #: (untyped) -> untyped
  def self.default_external=(enc)
    e = find(enc)
    __set_defaults(e, default_internal)
    enc
  end

  #: () -> Encoding?
  def self.default_internal = %x{
    if rbEncDefaultInternal == nil {
      return nil
    }
    return Ref(rbEncDefaultInternal)
  }

  # Stored, and encode with no target encodes to it, as MRI's does; IO does not convert to it (decision 136).
  #: (untyped) -> untyped
  def self.default_internal=(enc)
    e = enc.nil? ? nil : find(enc)
    __set_defaults(default_external, e)
    enc
  end

  #: (Encoding, Encoding?) -> void
  def self.__set_defaults(ext, intern) = %x{
    rbEncDefaultExternal, rbEncDefaultInternal = ext, nil
    if intern != nil {
      rbEncDefaultInternal = *intern
    }
  }

  # MRI's rb_enc_compatible over derived encodings: the encoding a concatenation of a and b would have, or nil.
  #: (untyped, untyped) -> Encoding?
  def self.compatible?(a, b)
    ea = __enc_of(a)
    eb = __enc_of(b)
    return nil unless ea && eb
    return ea if ea.equal?(eb)

    sa = a.is_a?(String)
    sb = b.is_a?(String)
    return ea if sb && b.to_s.empty?
    return (ea.ascii_compatible? && b.to_s.ascii_only? ? ea : eb) if sa && sb && a.to_s.empty?
    return nil unless ea.ascii_compatible? && eb.ascii_compatible?
    return ea if !sb && eb.equal?(US_ASCII)
    return eb if !sa && ea.equal?(US_ASCII)
    return nil unless sa || sb

    __compatible_str(sa ? a.to_s : b.to_s, sa ? ea : eb, sa ? eb : ea, sa && sb ? b.to_s : nil)
  end

  #: (String, Encoding, Encoding, String?) -> Encoding?
  def self.__compatible_str(s1, e1, e2, s2)
    a1 = s1.ascii_only?
    if s2
      a2 = s2.ascii_only?
      return e2 if a1 && !a2
      return e1 if a2
    end
    a1 ? e2 : nil
  end

  #: (untyped) -> Encoding?
  def self.__enc_of(x) = %x{
    switch v := rbUnbox(x).(type) {
    case *Encoding:
      return Ref(v)
    case String:
      if utf8.ValidString(string(v)) {
        return Ref(Encoding_UTF_8)
      }
      return Ref(Encoding_ASCII_8BIT)
    }
    return nil
  }

  # Raised only for the encodings and conversions decision 136 leaves out.
  class CompatibilityError < EncodingError; end

  class ConverterNotFoundError < EncodingError; end

  class UndefinedConversionError < EncodingError
    # @rbs @source_encoding_name: String?
    # @rbs @destination_encoding_name: String?
    # @rbs @error_char: String?

    #: (?String?, ?String?, ?String?, ?String?) -> void
    def initialize(message = nil, src = nil, dst = nil, char = nil)
      super(message)
      @source_encoding_name = src
      @destination_encoding_name = dst
      @error_char = char
    end

    #: () -> String?
    def source_encoding_name = @source_encoding_name

    #: () -> String?
    def destination_encoding_name = @destination_encoding_name

    #: () -> Encoding?
    def source_encoding = (n = @source_encoding_name) ? Encoding.find(n) : nil

    #: () -> Encoding?
    def destination_encoding = (n = @destination_encoding_name) ? Encoding.find(n) : nil

    #: () -> String?
    def error_char = @error_char
  end

  class InvalidByteSequenceError < EncodingError
    # @rbs @source_encoding_name: String?
    # @rbs @destination_encoding_name: String?
    # @rbs @error_bytes: String?
    # @rbs @readagain_bytes: String?
    # @rbs @incomplete: bool?

    #: (?String?, ?String?, ?String?, ?String?, ?String?, ?bool?) -> void
    def initialize(message = nil, src = nil, dst = nil, bytes = nil, again = nil, incomplete = nil)
      super(message)
      @source_encoding_name = src
      @destination_encoding_name = dst
      @error_bytes = bytes
      @readagain_bytes = again
      @incomplete = incomplete
    end

    #: () -> String?
    def source_encoding_name = @source_encoding_name

    #: () -> String?
    def destination_encoding_name = @destination_encoding_name

    #: () -> Encoding?
    def source_encoding = (n = @source_encoding_name) ? Encoding.find(n) : nil

    #: () -> Encoding?
    def destination_encoding = (n = @destination_encoding_name) ? Encoding.find(n) : nil

    #: () -> String?
    def error_bytes = @error_bytes

    #: () -> String?
    def readagain_bytes = @readagain_bytes

    #: () -> bool
    def incomplete_input? = @incomplete == true
  end
end

class String
  # Derived, not stored (decision 136): valid UTF-8 is UTF-8, anything else ASCII-8BIT.
  #: () -> Encoding
  def encoding = valid_encoding? ? Encoding::UTF_8 : Encoding::ASCII_8BIT

  # The bytes stay as they are; the name is still checked, as MRI's is.
  #: (untyped) -> String
  def force_encoding(enc)
    Encoding.find(enc)
    self
  end

  #: () -> bool
  def valid_encoding? = %x{ Boolean(utf8.ValidString(string(self))) }

  #: (?String?) -> String
  def scrub(repl = nil) = %x{
    r := "\\uFFFD"
    if repl != nil {
      r = string(*repl)
    }
    return rbNewStr(self, String(rbScrub(string(self), r)))
  }

  #: () { (String) -> String } -> String
  def __scrub_block
    parts = __scrub_parts
    out = ""
    i = 0
    while i < parts.size
      p = parts[i] || ""
      out += i.odd? ? yield(p) : p
      i += 1
    end
    out
  end

  #: () -> Array[String]
  def __scrub_parts = %x{ return rbStrs(rbScrubParts(string(self))) }

  #: (?untyped, ?untyped, ?invalid: Symbol?, ?undef: Symbol?, ?replace: String?, ?xml: Symbol?, ?universal_newline: bool?, ?crlf_newline: bool?, ?cr_newline: bool?) -> String
  def encode(to = nil, from = nil, invalid: nil, undef: nil, replace: nil, xml: nil, universal_newline: nil, crlf_newline: nil, cr_newline: nil) = %x{
    flag := func(b *Boolean) bool { return b != nil && bool(*b) }
    o := rbEncOptions(invalid, undef, replace, xml, flag(universal_newline), flag(crlf_newline), flag(cr_newline))
    dflt := ""
    if rbEncDefaultInternal != nil {
      dflt = rbEncDefaultInternal.name
    }
    return rbStrEncode(self, to, from, dflt, o)
  }

  #: (?Symbol) -> String
  def unicode_normalize(form = :nfc) = %x{ return String(rbUnicodeNormalize(string(self), string(form))) }

  #: (?Symbol) -> bool
  def unicode_normalized?(form = :nfc) = %x{ Boolean(rbUnicodeNormalize(string(self), string(form)) == string(self)) }
end

class Integer
  #: (untyped) -> String
  def __chr_1(enc) = %x{ return rbIntChr(int(self), enc) }
end
