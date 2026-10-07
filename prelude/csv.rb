# prelude/csv.rb
# rbs_inline: enabled

require_relative "date"
require_relative "stringio"
require_relative "strscan"
require_relative "forwardable"
#
# CSV's string and file parser/generator, as MRI's for the default dialect:
# an empty unquoted field is nil, a quoted one "", and a blank line an
# empty row. Options come as a Hash (decision 23): col_sep, quote_char,
# row_sep, skip_blanks, force_quotes. `headers: true` and `converters:`
# (decision 117) are chosen by the compiler from the literal options:
# a CSV::Table of CSV::Rows, and untyped fields. Always defined.

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
      row.s = append(row.s, r...)
      out.s = append(out.s, row)
    }
    return out
  }

  #: (String, ?Hash[Symbol, untyped]) -> Array[String?]?
  def self.parse_line(str, opts = {}) = parse(str, opts).first

  # `headers: true`: the first row names the columns (decision 117).
  #: (String, ?Hash[Symbol, untyped]) -> CSV::Table
  def self.__parse_headers(str, opts = {})
    rows = parse(str, opts)
    conv = __converters(opts)
    hdr = rows.first
    return Table.new([], []) unless hdr

    out = [] #: Array[CSV::Row]
    rows.drop(1).each do |r|
      n = hdr.size > r.size ? hdr.size : r.size
      heads = [] #: Array[String?]
      fields = [] #: Array[untyped]
      (0...n).each do |i|
        heads << hdr[i]
        fields << __convert(r[i], conv)
      end
      out << Row.new(heads, fields)
    end
    Table.new(hdr, out)
  end

  # `converters:` without headers: number-looking fields become Integers and Floats.
  #: (String, ?Hash[Symbol, untyped]) -> Array[Array[untyped]]
  def self.__parse_converted(str, opts = {})
    conv = __converters(opts)
    parse(str, opts).map { |r| r.map { |f| __convert(f, conv) } }
  end

  #: (String, ?Hash[Symbol, untyped]) -> Array[untyped]?
  def self.__parse_line_converted(str, opts = {}) = __parse_converted(str, opts).first

  #: (String, ?Hash[Symbol, untyped]) -> CSV::Table
  def self.__read_headers(path, opts = {}) = __parse_headers(File.read(path), opts)

  #: (String, ?Hash[Symbol, untyped]) -> Array[Array[untyped]]
  def self.__read_converted(path, opts = {}) = __parse_converted(File.read(path), opts)

  #: (String, ?Hash[Symbol, untyped]) { (CSV::Row) -> void } -> void
  def self.__foreach_headers(path, opts = {})
    __read_headers(path, opts).each { |row| yield row }
  end

  #: (String, ?Hash[Symbol, untyped]) { (Array[untyped]) -> void } -> void
  def self.__foreach_converted(path, opts = {})
    __read_converted(path, opts).each { |row| yield row }
  end

  # The converter names, checked by the compiler: :integer, :float, or :numeric for both.
  #: (Hash[Symbol, untyped]) -> Array[Symbol]
  def self.__converters(opts)
    out = [] #: Array[Symbol]
    c = opts[:converters]
    names = [] #: Array[untyped]
    names << c if c && !c.is_a?(Array)
    c.each { |x| names << x } if c.is_a?(Array)
    names.each do |n|
      case n
      when :numeric
        out << :integer
        out << :float
      when :integer then out << :integer
      when :float then out << :float
      end
    end
    out
  end

  # MRI's :integer is Integer(field) and :float Float(field), each kept when it raises.
  #: (String?, Array[Symbol]) -> untyped
  def self.__convert(field, conv)
    return field if field.nil? || conv.empty?

    if conv.include?(:integer)
      begin
        return Integer(field)
      rescue ArgumentError
        nil
      end
    end
    if conv.include?(:float)
      begin
        return Float(field)
      rescue ArgumentError
        nil
      end
    end
    field
  end

  # One data row read with headers: fields by header or position.
  class Row < Object
    attr_reader :headers #: Array[String?]
    attr_reader :fields #: Array[untyped]

    #: (Array[String?], Array[untyped]) -> void
    def initialize(headers, fields)
      @headers = headers
      @fields = fields
    end

    # The field under header key (the first, when headers repeat), or at index key.
    #: (untyped) -> untyped
    def [](key)
      return @fields[key] if key.is_a?(Integer)

      i = @headers.index { |h| h == key }
      i ? @fields[i] : nil
    end

    #: (untyped) -> untyped
    def field(key) = self[key]

    #: (untyped) -> untyped
    def fetch(key)
      i = @headers.index { |h| h == key }
      raise KeyError.__for("key not found: #{key}", self, key) unless i

      @fields[i]
    end

    #: (untyped) -> bool
    def header?(key) = @headers.any? { |h| h == key }

    #: (untyped) -> bool
    def has_key?(key) = header?(key)

    #: () -> Hash[String?, untyped]
    def to_h
      out = {} #: Hash[String?, untyped]
      @headers.each_with_index { |h, i| out[h] = @fields[i] unless out.key?(h) }
      out
    end

    #: () { (String?, untyped) -> void } -> void
    def each
      @headers.each_with_index { |h, i| yield h, @fields[i] }
    end

    #: () -> Integer
    def size = @fields.size

    #: () -> String
    def to_s = CSV.generate_line(@fields)

    #: () -> String
    def inspect
      pairs = @headers.each_with_index.map { |h, i| "#{h.inspect}:#{@fields[i].inspect}" }
      "#<CSV::Row #{pairs.join(" ")}>"
    end

    #: (untyped) -> bool
    def ==(other)
      return false unless other.is_a?(Row)

      o = other #: CSV::Row
      o.headers == headers && o.fields == fields
    end
  end

  # The rows of a CSV read with headers.
  # Not Enumerable: MRI's to_a is the rows as Arrays, header row first, where Enumerable's would be the Rows.
  class Table < Object
    attr_reader :headers #: Array[String?]

    #: (Array[String?], Array[CSV::Row]) -> void
    def initialize(headers, rows)
      @headers = headers
      @rows = rows
    end

    #: () { (CSV::Row) -> void } -> void
    def each
      @rows.each { |r| yield r }
    end

    # @rbs [U] () { (CSV::Row) -> U } -> Array[U]
    def map(&blk) = @rows.map(&blk)

    #: () { (CSV::Row) -> bool } -> Array[CSV::Row]
    def select(&blk) = @rows.select(&blk)

    #: () { (CSV::Row) -> bool } -> Array[CSV::Row]
    def reject(&blk) = @rows.reject(&blk)

    #: () { (CSV::Row) -> bool } -> CSV::Row?
    def find(&blk) = @rows.find(&blk)

    #: () { (CSV::Row, Integer) -> void } -> void
    def each_with_index
      @rows.each_with_index { |r, i| yield r, i }
    end

    #: () -> CSV::Row?
    def first = @rows.first

    #: () -> Integer
    def count = @rows.size

    #: () -> Array[CSV::Row]
    def rows = @rows

    # A Row for an Integer, a column's values for a header.
    #: (untyped) -> untyped
    def [](key)
      return @rows[key] if key.is_a?(Integer)

      @rows.map { |r| r[key] }
    end

    #: () -> Integer
    def size = @rows.size

    #: () -> Integer
    def length = @rows.size

    #: () -> Array[Array[untyped]]
    def to_a
      out = [] #: Array[Array[untyped]]
      head = [] #: Array[untyped]
      @headers.each { |h| head << h }
      out << head
      @rows.each { |r| out << r.fields }
      out
    end

    #: () -> String
    def to_s = ([CSV.generate_line(@headers)] + @rows.map(&:to_s)).join

    #: () -> String
    def inspect = "#<CSV::Table mode:col_or_row row_count:#{@rows.size + 1}>\n#{self}"
  end

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
    return String(rbCSVLine(row.s, rbCSVStrOpt(opts, "col_sep", ","), rbCSVStrOpt(opts, "quote_char", "\\""), rbCSVStrOpt(opts, "row_sep", "\\n"), rbCSVBoolOpt(opts, "force_quotes")))
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
    line := rbCSVLine(row.s, self.sep, self.quote, self.rowSep, self.force)
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
