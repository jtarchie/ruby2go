# rbs_inline: enabled
#
# OptionParser (decision 101): MRI's optparse over a Go engine
# (prelude/go/optparse.go). `on`'s block is typed at compile time from its
# literal switch strings and coercion class: the compiler sends each call to
# one of the `__on_<kind>` variants below.

# @go_type struct { banner *string; head, items, tail []rbOptItem; width int; indent, prog, version string; off map[string]bool }
class OptionParser < Object
  NO_ARGUMENT = :NONE
  REQUIRED_ARGUMENT = :REQUIRED
  OPTIONAL_ARGUMENT = :OPTIONAL

  #: (?String?) -> OptionParser
  def self.new(banner = nil) = __make(File.basename($0), banner)

  #: (?String?) { (OptionParser) -> void } -> OptionParser
  def self.__new_block(banner = nil)
    parser = __make(File.basename($0), banner)
    yield parser
    parser
  end

  #: (String, String?) -> OptionParser
  def self.__make(prog, banner) = %x{ return rbNewOptionParser(string(prog), banner) }

  # A switch whose strings aren't all literals: its kind is read at run
  # time from the strings alone (a flag or a String argument).
  #: (*untyped) ?{ (untyped) -> void } -> OptionParser
  def on(*specs) = %x{ return self.rbOn(0, "flag", rest_, func(v any) { if blk != nil { blk(rbOptStored(v)) } }) }

  #: (*untyped) ?{ (untyped) -> void } -> OptionParser
  def on_tail(*specs) = %x{ return self.rbOn(1, "flag", rest_, func(v any) { if blk != nil { blk(rbOptStored(v)) } }) }

  #: (*untyped) ?{ (untyped) -> void } -> OptionParser
  def on_head(*specs) = %x{ return self.rbOn(-1, "flag", rest_, func(v any) { if blk != nil { blk(rbOptStored(v)) } }) }

  #: (*untyped) ?{ (bool) -> void } -> OptionParser
  def __on_flag(*specs) = %x{ return self.rbOn(0, "flag", rest_, func(v any) { if blk != nil { blk(v.(Boolean)) } }) }

  #: (*untyped) ?{ (String) -> void } -> OptionParser
  def __on_string(*specs) = %x{ return self.rbOn(0, "string", rest_, func(v any) { if blk != nil { blk(v.(String)) } }) }

  #: (*untyped) ?{ (String?) -> void } -> OptionParser
  def __on_string_opt(*specs) = %x{ return self.rbOn(0, "string_opt", rest_, func(v any) { if blk != nil { blk(v.(*String)) } }) }

  #: (*untyped) ?{ (Integer) -> void } -> OptionParser
  def __on_integer(*specs) = %x{ return self.rbOn(0, "integer", rest_, func(v any) { if blk != nil { blk(v.(Integer)) } }) }

  #: (*untyped) ?{ (Integer?) -> void } -> OptionParser
  def __on_integer_opt(*specs) = %x{ return self.rbOn(0, "integer_opt", rest_, func(v any) { if blk != nil { blk(v.(*Integer)) } }) }

  #: (*untyped) ?{ (Float) -> void } -> OptionParser
  def __on_float(*specs) = %x{ return self.rbOn(0, "float", rest_, func(v any) { if blk != nil { blk(v.(Float)) } }) }

  #: (*untyped) ?{ (Float?) -> void } -> OptionParser
  def __on_float_opt(*specs) = %x{ return self.rbOn(0, "float_opt", rest_, func(v any) { if blk != nil { blk(v.(*Float)) } }) }

  #: (*untyped) ?{ (Array[String]) -> void } -> OptionParser
  def __on_array(*specs) = %x{ return self.rbOn(0, "array", rest_, func(v any) { if blk != nil { blk(v.(*Array[String])) } }) }

  #: (*untyped) ?{ (bool) -> void } -> OptionParser
  def __on_tail_flag(*specs) = %x{ return self.rbOn(1, "flag", rest_, func(v any) { if blk != nil { blk(v.(Boolean)) } }) }

  #: (*untyped) ?{ (String) -> void } -> OptionParser
  def __on_tail_string(*specs) = %x{ return self.rbOn(1, "string", rest_, func(v any) { if blk != nil { blk(v.(String)) } }) }

  #: (*untyped) ?{ (String?) -> void } -> OptionParser
  def __on_tail_string_opt(*specs) = %x{ return self.rbOn(1, "string_opt", rest_, func(v any) { if blk != nil { blk(v.(*String)) } }) }

  #: (*untyped) ?{ (Integer) -> void } -> OptionParser
  def __on_tail_integer(*specs) = %x{ return self.rbOn(1, "integer", rest_, func(v any) { if blk != nil { blk(v.(Integer)) } }) }

  #: (*untyped) ?{ (Integer?) -> void } -> OptionParser
  def __on_tail_integer_opt(*specs) = %x{ return self.rbOn(1, "integer_opt", rest_, func(v any) { if blk != nil { blk(v.(*Integer)) } }) }

  #: (*untyped) ?{ (Float) -> void } -> OptionParser
  def __on_tail_float(*specs) = %x{ return self.rbOn(1, "float", rest_, func(v any) { if blk != nil { blk(v.(Float)) } }) }

  #: (*untyped) ?{ (Float?) -> void } -> OptionParser
  def __on_tail_float_opt(*specs) = %x{ return self.rbOn(1, "float_opt", rest_, func(v any) { if blk != nil { blk(v.(*Float)) } }) }

  #: (*untyped) ?{ (Array[String]) -> void } -> OptionParser
  def __on_tail_array(*specs) = %x{ return self.rbOn(1, "array", rest_, func(v any) { if blk != nil { blk(v.(*Array[String])) } }) }

  #: (*untyped) ?{ (bool) -> void } -> OptionParser
  def __on_head_flag(*specs) = %x{ return self.rbOn(-1, "flag", rest_, func(v any) { if blk != nil { blk(v.(Boolean)) } }) }

  #: (*untyped) ?{ (String) -> void } -> OptionParser
  def __on_head_string(*specs) = %x{ return self.rbOn(-1, "string", rest_, func(v any) { if blk != nil { blk(v.(String)) } }) }

  #: (*untyped) ?{ (String?) -> void } -> OptionParser
  def __on_head_string_opt(*specs) = %x{ return self.rbOn(-1, "string_opt", rest_, func(v any) { if blk != nil { blk(v.(*String)) } }) }

  #: (*untyped) ?{ (Integer) -> void } -> OptionParser
  def __on_head_integer(*specs) = %x{ return self.rbOn(-1, "integer", rest_, func(v any) { if blk != nil { blk(v.(Integer)) } }) }

  #: (*untyped) ?{ (Integer?) -> void } -> OptionParser
  def __on_head_integer_opt(*specs) = %x{ return self.rbOn(-1, "integer_opt", rest_, func(v any) { if blk != nil { blk(v.(*Integer)) } }) }

  #: (*untyped) ?{ (Float) -> void } -> OptionParser
  def __on_head_float(*specs) = %x{ return self.rbOn(-1, "float", rest_, func(v any) { if blk != nil { blk(v.(Float)) } }) }

  #: (*untyped) ?{ (Float?) -> void } -> OptionParser
  def __on_head_float_opt(*specs) = %x{ return self.rbOn(-1, "float_opt", rest_, func(v any) { if blk != nil { blk(v.(*Float)) } }) }

  #: (*untyped) ?{ (Array[String]) -> void } -> OptionParser
  def __on_head_array(*specs) = %x{ return self.rbOn(-1, "array", rest_, func(v any) { if blk != nil { blk(v.(*Array[String])) } }) }

  #: (String) -> OptionParser
  def separator(line) = %x{
    self.items = append(self.items, rbOptItem{sep: string(line)})
    return self
  }

  #: () -> String
  def banner = %x{
    if self.banner != nil {
      return String(*self.banner)
    }
    return String("Usage: " + self.prog + " [options]")
  }

  #: (String) -> String
  def banner=(text)
    %x{
    b := string(text)
    self.banner = &b
    return text
    }
  end

  #: () -> String
  def program_name = %x{ String(self.prog) }

  #: (String) -> String
  def program_name=(name)
    %x{
    self.prog = string(name)
    return name
    }
  end

  #: () -> String?
  def version = %x{
    if self.version == "" {
      return nil
    }
    v := String(self.version)
    return &v
  }

  #: (String) -> String
  def version=(v)
    %x{
    self.version = string(v)
    return v
    }
  end

  #: () -> Integer
  def summary_width = %x{ Integer(self.width) }

  #: (Integer) -> Integer
  def summary_width=(w)
    %x{
    self.width = int(w)
    return w
    }
  end

  #: () -> String
  def summary_indent = %x{ String(self.indent) }

  #: (String) -> String
  def summary_indent=(text)
    %x{
    self.indent = string(text)
    return text
    }
  end

  # The default list: only its long table, whose delete turns off --help or --version (decision 245).
  #: () -> OptionParser::List
  def base = OptionParser::List.new(self)

  #: (String) -> nil
  def __no_officious(name) = %x{
    if self.off == nil {
      self.off = map[string]bool{}
    }
    self.off[string(name)] = true
  }

  # A switch from on's arguments, not added to the parser: [switch] (MRI's array has more after it).
  #: (Array[untyped], ?untyped) -> Array[OptionParser::Switch]
  def make_switch(opts, _block = nil)
    sw = OptionParser::Switch.new
    sw.__fill(opts)
    [sw]
  end

  #: () -> String
  def help = %x{ String(self.rbHelp()) }

  #: () -> String
  def to_s = help

  #: () -> String
  def inspect = "#<OptionParser>"

  # Switches anywhere, as MRI's default permute mode; returns the other arguments.
  #: (Array[String]) -> Array[String]
  def parse(argv) = __parse_copy(argv, nil)

  #: (Array[String], Hash[Symbol, untyped]) -> Array[String]
  def __parse_2(argv, opts) = __parse_copy(argv, opts[:into])

  #: (Array[String], untyped) -> Array[String]
  def __parse_copy(argv, into) = %x{
    return &Array[String]{s: self.rbParse(argv.s, into)}
  }

  # Removes the switches from argv (ARGV by default) and returns it.
  #: (?Array[String]) -> Array[String]
  def parse!(argv = ARGV) = %x{
    argv.s = self.rbParse(argv.s, nil)
    return argv
  }

  #: (Array[String], Hash[Symbol, untyped]) -> Array[String]
  def __parse_bang_2(argv, opts) = __parse_into(argv, opts[:into])

  #: (Hash[Symbol, untyped]) -> Array[String]
  def __parse_bang_hash(opts) = __parse_into(ARGV, opts[:into])

  #: (Array[String], untyped) -> Array[String]
  def __parse_into(argv, into) = %x{
    argv.s = self.rbParse(argv.s, into)
    return argv
  }
end

class OptionParser::ParseError < RuntimeError; end
class OptionParser::InvalidOption < OptionParser::ParseError; end
class OptionParser::MissingArgument < OptionParser::ParseError; end
class OptionParser::InvalidArgument < OptionParser::ParseError; end
class OptionParser::NeedlessArgument < OptionParser::ParseError; end
class OptionParser::AmbiguousOption < OptionParser::ParseError; end

# OptionParser#base's long table: delete("help") / delete("version") turns off that built-in switch.
class OptionParser::List
  #: (OptionParser) -> void
  def initialize(parser) = @parser = parser

  #: () -> OptionParser::LongTable
  def long = OptionParser::LongTable.new(@parser)
end

class OptionParser::LongTable
  #: (OptionParser) -> void
  def initialize(parser) = @parser = parser

  #: (String) -> nil
  def delete(name) = @parser.__no_officious(name)
end

# What make_switch returns: a switch's strings as MRI's Switch holds them, and its help lines.
class OptionParser::Switch
  #: () -> void
  def initialize
    @short = [] #: Array[String]
    @long = [] #: Array[String]
    @desc = [] #: Array[String]
    @arg = nil #: String?
  end

  attr_reader :short, :long, :desc, :arg

  #: (Array[untyped]) -> void
  def __fill(opts) = %x{
    sw := rbOptBuild("flag", opts._ToAny().s)
    for _, x := range sw.short {
      self._OptionParser_Switch().short.s = append(self._OptionParser_Switch().short.s, String(x))
    }
    for _, x := range sw.long {
      self._OptionParser_Switch().long.s = append(self._OptionParser_Switch().long.s, String(x))
    }
    for _, x := range sw.desc {
      self._OptionParser_Switch().desc.s = append(self._OptionParser_Switch().desc.s, String(x))
    }
    if sw.arg != "" {
      self._OptionParser_Switch().arg = Ref(String(sw.arg))
    }
  }

  # MRI's Switch#summarize: names already in sdone/ldone are left out (and marked done), each line indented.
  #: (Hash[untyped, untyped], Hash[untyped, untyped], Integer, Integer, String) { (String) -> void } -> void
  def summarize(sdone, ldone, width, _max, indent)
    sopts = @short.reject { |s| sdone.key?(s) }
    lopts = @long.reject { |l| ldone.key?(l) }
    @short.each { |s| sdone[s] = true }
    @long.each { |l| ldone[l] = true }
    return if sopts.empty? && lopts.empty?

    __lines(sopts, lopts, width).each { |line| yield indent + line }
  end

  #: (Array[String], Array[String], Integer) -> Array[String]
  def __lines(sopts, lopts, width) = %x{
    sw := &rbOptSwitch{}
    for _, x := range sopts.s {
      sw.short = append(sw.short, string(x))
    }
    for _, x := range lopts.s {
      sw.long = append(sw.long, string(x))
    }
    for _, x := range self._OptionParser_Switch().desc.s {
      sw.desc = append(sw.desc, string(x))
    }
    if a := self._OptionParser_Switch().arg; a != nil {
      sw.arg = string(*a)
    }
    out := &Array[String]{}
    for _, l := range sw.summarize(int(width)) {
      out.s = append(out.s, String(l))
    }
    return out
  }
end
