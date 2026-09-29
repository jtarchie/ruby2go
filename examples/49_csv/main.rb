# rbs_inline: enabled
# args: --seed 1

require "csv"
require "minitest/autorun"

# Reads an inventory export, totals it per category, and writes a report back out as CSV.
class Inventory
  #: (String) -> void
  def initialize(data)
    rows = CSV.parse(data)
    @header = rows.shift || [] #: Array[String?]
    @rows = rows.reject(&:empty?)
  end

  #: (String) -> Integer
  def column(name) = @header.index(name) || raise(ArgumentError, "no column #{name}")

  #: () -> Hash[String, Integer]
  def totals
    cat = column("category")
    qty = column("qty")
    out = Hash.new #: Hash[String, Integer]
    @rows.each do |r|
      key = r[cat] || "(none)"
      out[key] = (out[key] || 0) + (r[qty] || "0").to_i
    end
    out
  end
end

DATA_CSV = <<~CSV
  sku,name,category,qty
  A1,"Widget, large",tools,4
  A2,"The ""best"" gadget",gadgets,7

  A3,Plain,tools,
  A4,"multi
  line",,2
CSV

class CSVTest < Minitest::Test
  def test_parse_totals_and_generate_a_report
    inv = Inventory.new(DATA_CSV)
    report = CSV.generate do |csv|
      csv << ["category", "total"]
      inv.totals.each { |k, v| csv << [k, v] }
    end
    assert_equal "category,total\ntools,4\ngadgets,7\n(none),2\n", report
  end

  def test_quoted_fields_span_lines_and_empty_fields_are_nil
    assert_equal ["A4", "multi\nline", nil, "2"], CSV.parse(DATA_CSV).last
  end

  def test_parse_line_and_parse_csv
    assert_equal ["1", "2", nil, "x;y"], CSV.parse_line("1;2;;\"x;y\"", col_sep: ";")
    assert_nil CSV.parse_line("")
    assert_equal ["a", "", "b"], "a,\"\",b".parse_csv
    assert_equal [["a", "b"], ["c", "d"]], CSV.parse("a,b\r\nc,d\r\n")
  end

  def test_generate_line_quotes_only_what_needs_it
    assert_equal "a,,\"\",\"b,c\",\"q\"\"t\",1,2.5, sp,\"nl\n\"\n",
                 CSV.generate_line(["a", nil, "", "b,c", "q\"t", 1, 2.5, " sp", "nl\n"])
    assert_equal "x,y\n", ["x", "y"].to_csv
    assert_equal "1,,sym\n", [1, nil, :sym].to_csv
    assert_equal "a\t\"b\tc\"\n", CSV.generate_line(["a", "b\tc"], col_sep: "\t")
    assert_equal "\"a|b\"|c\n", CSV.generate(col_sep: "|") { |c| c << ["a|b", "c"] }
  end

  def test_malformed_input_raises
    e = assert_raises(CSV::MalformedCSVError) { CSV.parse("ok\n\"abc") }
    assert_equal "Unclosed quoted field in line 2.", e.message
    e = assert_raises(CSV::MalformedCSVError) { CSV.parse("a\"b\"c") }
    assert_equal "Illegal quoting in line 1.", e.message
  end
end
