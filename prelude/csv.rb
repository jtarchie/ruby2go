# prelude/csv.rb
# rbs_inline: enabled
#
# CSV's string parser and generator, as MRI's for the default dialect:
# an empty unquoted field is nil, a quoted one "", and a blank line an
# empty row. Options come as a Hash (decision 23): col_sep and quote_char.
# Always defined.

# @go_type struct { buf strings.Builder; sep string; quote string }
class CSV < Object
  class MalformedCSVError < RuntimeError; end

  #: (String, ?Hash[Symbol, String]) -> Array[Array[String?]]
  def self.parse(str, opts = {}) = %x{
    rows, err := rbCSVParse(string(str), rbCSVOpt(opts, "col_sep", ","), rbCSVOpt(opts, "quote_char", "\\""))
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

  #: (String, ?Hash[Symbol, String]) -> Array[String?]?
  def self.parse_line(str, opts = {}) = parse(str, opts).first

  #: (Array[untyped], ?Hash[Symbol, String]) -> String
  def self.generate_line(row, opts = {}) = %x{
    return String(rbCSVLine(*row, rbCSVOpt(opts, "col_sep", ","), rbCSVOpt(opts, "quote_char", "\\"")))
  }

  #: (?Hash[Symbol, String]) { (CSV) -> void } -> String
  def self.generate(opts = {})
    csv = __new(opts)
    yield csv
    csv.string
  end

  #: (Hash[Symbol, String]) -> CSV
  def self.__new(opts) = %x{ return &CSV{sep: rbCSVOpt(opts, "col_sep", ","), quote: rbCSVOpt(opts, "quote_char", "\\"")} }

  #: (Array[untyped]) -> CSV
  def <<(row) = %x{
    self.buf.WriteString(rbCSVLine(*row, self.sep, self.quote))
    return self
  }

  #: (Array[untyped]) -> CSV
  def add_row(row) = self << row

  #: () -> String
  def string = %x{ String(self.buf.String()) }
end

class Array
  #: (?Hash[Symbol, String]) -> String
  def to_csv(opts = {}) = CSV.generate_line(self, opts)
end

class String
  #: (?Hash[Symbol, String]) -> Array[String?]?
  def parse_csv(opts = {}) = CSV.parse_line(self, opts)
end
