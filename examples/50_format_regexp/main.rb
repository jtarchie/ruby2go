# rbs_inline: enabled
# args: --seed 1

require "singleton"
require "minitest/autorun"

# A process-wide metrics registry that parses "name=value" samples and renders a table.
class Metrics
  include Singleton

  #: () -> void
  def initialize
    @samples = {} #: Hash[String, Array[Float]]
  end

  #: (String) -> Integer
  def ingest(text)
    pairs = text.scan(/(\w+)=(-?\d+(?:\.\d+)?)/)
    pairs.each do |name, value|
      (@samples[name || ""] ||= []) << (value || "0").to_f
    end
    pairs.size
  end

  #: () -> String
  def report
    lines = [format("%-10s %5s %8s %8s", "metric", "n", "mean", "max")]
    @samples.keys.sort.each do |name|
      vals = @samples.fetch(name)
      lines << format("%-10s %5d %8.2f %8.2f", name, vals.size, vals.sum / vals.size, vals.max || 0.0)
    end
    lines.join("\n")
  end
end

class MetricsTest < Minitest::Test
  # The registry is one shared instance, so ingest and report live in one test.
  def test_ingest_samples_and_render_a_table
    assert_equal 4, Metrics.instance.ingest("cpu=0.5 mem=512 cpu=0.75 latency_ms=12.5")
    assert_equal 3, Metrics.instance.ingest("mem=640, latency_ms=-3; junk cpu=1")
    assert_equal <<~TABLE.chomp, Metrics.instance.report
      metric         n     mean      max
      cpu            3     0.75     1.00
      latency_ms     2     4.75    12.50
      mem            2   576.00   640.00
    TABLE
  end

  def test_singleton_instance_is_shared
    assert Metrics.instance.equal?(Metrics.instance)
  end
end

class RegexpTest < Minitest::Test
  def test_gsub_sub_and_block_replacement
    slug = "  Hello, World! Ruby->Go  ".strip.downcase.gsub(/[^a-z0-9]+/, "-").gsub(/\A-|-\z/, "")
    assert_equal "hello-world-ruby-go", slug
    assert_equal "05/03/2024", "2024-03-05".sub(/(\d+)-(\d+)-(\d+)/, "\\3/\\2/\\1")
    assert_equal "snakeCaseName", "snake_case_name".gsub(/_(\w)/) { |m| m[1].to_s.upcase }
  end

  def test_scan_and_split
    assert_equal ["1999", "2024"], "The year 1999 and 2024".scan(/\d{4}/)
    assert_equal ["a", "b", "c", "d"], "a, b;c  d".split(/[,;\s]+/)
  end
end

class FormatTest < Minitest::Test
  def test_string_percent_and_format
    assert_equal "ann scored 093.5% (+7)", "%s scored %05.1f%% (%+d)" % ["ann", 93.456, 7]
    assert_equal "Bo is 3", format("%<name>s is %<age>d", name: "Bo", age: 3)
    assert_equal "ff 10 101 1.234500e+03", format("%x %o %b %e", 255, 8, 5, 1234.5)
    assert_equal "  7|7  |", format("%3d|%-3d|", 7, 7)
  end
end
