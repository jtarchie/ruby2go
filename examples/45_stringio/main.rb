# rbs_inline: enabled
# args: --seed 1

require "stringio"
require "minitest/autorun"

# A report renderer that writes to any StringIO, then a parser that reads it back.
class Report
  #: (Array[[String, Integer]]) -> void
  def initialize(rows)
    @rows = rows
  end

  #: (StringIO) -> Integer
  def render(out)
    out.puts "name,qty"
    @rows.each { |name, qty| out << name << "," << qty << "\n" }
    out.print "total,", @rows.sum { |_, q| q }, "\n"
    @rows.size
  end
end

CSV_TEXT = "name,qty\napple,3\npear,5\nfig,12\ntotal,20\n"

class StringIOTest < Minitest::Test
  def test_render_writes_with_puts_append_and_print
    buf = StringIO.new
    count = Report.new([["apple", 3], ["pear", 5], ["fig", 12]]).render(buf)
    assert_equal 3, count
    assert_equal 40, buf.size
    assert_equal CSV_TEXT, buf.string
  end

  def test_gets_and_each_line_read_it_back
    input = StringIO.new(CSV_TEXT)
    header = input.gets || ""
    assert_equal "name,qty", header.chomp
    totals = Hash.new #: Hash[String, Integer]
    input.each_line do |line|
      name, qty = line.chomp.split(",")
      totals[name || ""] = (qty || "0").to_i
    end
    assert_equal [["apple", 3], ["pear", 5], ["fig", 12], ["total", 20]], totals.to_a
    assert_equal true, input.eof?
    assert_equal 5, input.lineno
  end

  def test_rewind_read_getc_and_readlines
    input = StringIO.new(CSV_TEXT)
    input.read
    input.rewind
    assert_equal "name", input.read(4)
    assert_equal ",", input.getc
    assert_equal 5, input.readlines.size
    assert_nil input.read(1)
  end

  def test_write_overwrites_at_the_position
    patch = StringIO.new("hello world")
    patch.write("J")
    patch.pos = 6
    patch.write("W")
    assert_equal "Jello World", patch.string
  end
end
