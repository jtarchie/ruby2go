# prelude/csv.rb
# rbs_inline: enabled
#
# CSV's string and file parser/generator, as MRI's for the default dialect:
# an empty unquoted field is nil, a quoted one "", and a blank line an
# empty row. Options come as a Hash (decision 23): col_sep, quote_char,
# row_sep, skip_blanks, force_quotes. headers:/converters: are not
# recognized (decision 53). Always defined.

# @go_type struct { buf strings.Builder; sep string; quote string; rowSep string; force bool; file *File }
class CSV < Object
  class MalformedCSVError < RuntimeError; end

  #: (String, ?Hash[Symbol, untyped]) -> Array[Array[String?]]
  def self.parse(str, opts = {}) = %x{
    rows, err := rbCSVParse(string(str), rbCSVStrOpt(opts, "col_sep", ","), rbCSVStrOpt(opts, "quote_char", "\\""), rbCSVStrOpt(opts, "row_sep", ""), rbCSVBoolOpt(opts, "skip_blanks"))
    if err != "" {
      panic(NewCSV_MalformedCSVError(Ref(String(err))))
    }
    out := &Array[*Array[*String]]{}
    for _, r := range rows {
      row := &Array[*String]{}
      *row = append(*row, r...)
      *out = append(*out, row)
    }
    return out
  }

  #: (String, ?Hash[Symbol, untyped]) -> Array[String?]?
  def self.parse_line(str, opts = {}) = parse(str, opts).first

  #: (String, ?Hash[Symbol, untyped]) -> Array[Array[String?]]
  def self.read(path, opts = {}) = parse(File.read(path), opts)

  # ponytail: reads the whole file (parse works on a string), then iterates; fine for typical CSVs, revisit with a streaming row reader if multi-GB files matter.
  #: (String, ?Hash[Symbol, untyped]) { (Array[String?]) -> void } -> void
  def self.foreach(path, opts = {})
    read(path, opts).each { |row| yield row }
  end

  #: (String, ?Hash[Symbol, untyped]) -> Array[Array[String?]]
  def self.__foreach_enum(path, opts = {}) = read(path, opts)

  #: (Array[untyped], ?Hash[Symbol, untyped]) -> String
  def self.generate_line(row, opts = {}) = %x{
    return String(rbCSVLine(*row, rbCSVStrOpt(opts, "col_sep", ","), rbCSVStrOpt(opts, "quote_char", "\\""), rbCSVStrOpt(opts, "row_sep", "\\n"), rbCSVBoolOpt(opts, "force_quotes")))
  }

  #: (?Hash[Symbol, untyped]) { (CSV) -> void } -> String
  def self.generate(opts = {})
    csv = __new(opts)
    yield csv
    csv.string
  end

  #: [T] (String, ?String, ?Hash[Symbol, untyped]) { (CSV) -> T } -> T
  def self.open(path, mode = "r", opts = {})
    f = File.new(path, mode)
    csv = __new_file(f, opts)
    begin
      yield csv
    ensure
      f.close
    end
  end

  #: (Hash[Symbol, untyped]) -> CSV
  def self.__new(opts) = %x{
    return &CSV{
      sep:    rbCSVStrOpt(opts, "col_sep", ","),
      quote:  rbCSVStrOpt(opts, "quote_char", "\\""),
      rowSep: rbCSVStrOpt(opts, "row_sep", "\\n"),
      force:  rbCSVBoolOpt(opts, "force_quotes"),
    }
  }

  #: (File, Hash[Symbol, untyped]) -> CSV
  def self.__new_file(f, opts) = %x{
    return &CSV{
      sep:    rbCSVStrOpt(opts, "col_sep", ","),
      quote:  rbCSVStrOpt(opts, "quote_char", "\\""),
      rowSep: rbCSVStrOpt(opts, "row_sep", "\\n"),
      force:  rbCSVBoolOpt(opts, "force_quotes"),
      file:   f,
    }
  }

  #: (Array[untyped]) -> CSV
  def <<(row) = %x{
    line := rbCSVLine(*row, self.sep, self.quote, self.rowSep, self.force)
    if self.file != nil {
      if self.file.w == nil {
        panic(NewIOError(Ref[String]("not opened for writing")))
      }
      _, _ = self.file.w.WriteString(line)
      return self
    }
    self.buf.WriteString(line)
    return self
  }

  #: (Array[untyped]) -> CSV
  def add_row(row) = self << row

  #: () -> String
  def string = %x{ String(self.buf.String()) }
end

class Array
  #: (?Hash[Symbol, untyped]) -> String
  def to_csv(opts = {}) = CSV.generate_line(self, opts)
end

class String
  #: (?Hash[Symbol, untyped]) -> Array[String?]?
  def parse_csv(opts = {}) = CSV.parse_line(self, opts)
end
