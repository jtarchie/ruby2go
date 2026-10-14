# rbs_inline: enabled

require "minitest/autorun"
require "base64"
require "csv"
require "json"
require "securerandom"
require "stringio"
require "strscan"
require "tempfile"
require "tmpdir"

# #101 stdlib hardening: hostile or degenerate arguments that hung, panicked or were silently wrong. Every expectation is MRI's.
module StdlibHardeningTests
  # The exception class an argument error check raises, "none" if it returns.
  #: () { () -> untyped } -> String
  def self.raised
    yield
    "none"
  rescue Exception => e # rubocop:disable Lint/RescueException
    e.class.to_s
  end

  class CsvTest < Minitest::Test
    # an empty col_sep made the parser append nil fields forever; an empty quote_char looped in the quoted-field scan
    def test_parse_rejects_an_empty_col_sep
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse("a,b", col_sep: "") }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse_line("a,b", col_sep: "") }
    end

    def test_parse_rejects_a_quote_char_that_is_not_one_character
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse("a,b", quote_char: "") }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse("a,b", quote_char: "ab") }
    end

    def test_parse_still_takes_a_multi_character_col_sep
      assert_equal [["a", "b"]], CSV.parse("a::b", col_sep: "::")
    end

    # the generator never raises: an empty col_sep joins the fields, an empty quote_char quotes nothing
    def test_generate_with_empty_separators
      assert_equal "a b1\n", CSV.generate_line(["a b", 1], col_sep: "")
      assert_equal "\"a\"\"b\"\"\"\n", CSV.generate_line(["a\"b", nil, ""], col_sep: "")
      assert_equal "a,b,1\n", CSV.generate_line(["a,b", 1], quote_char: "")
      assert_equal "a\"b,,\n", CSV.generate_line(["a\"b", nil, ""], quote_char: "")
      assert_equal "12\n", CSV.generate(col_sep: "") { |csv| csv << [1, 2] }
    end
  end

  class SecureRandomTest < Minitest::Test
    def test_negative_sizes_raise_argument_error
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.hex(-1) }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.random_bytes(-1) }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.base64(-1) }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.urlsafe_base64(-1) }
    end

    def test_zero_and_negative_alphanumeric_are_empty
      assert_equal "", SecureRandom.hex(0)
      assert_equal "", SecureRandom.random_bytes(0)
      assert_equal "", SecureRandom.alphanumeric(0)
      assert_equal "", SecureRandom.alphanumeric(-1)
    end
  end

  class Base64Test < Minitest::Test
    # Go's Strict() decoder skips \r and \n; MRI's m0 does not
    def test_strict_decode_rejects_whitespace_and_garbage
      ["YQ==\n", "YQ==\r", "YQ=\n=", "Y Q==", "YQ-_", "YQ", "Y\nQ=="].each do |s|
        assert_equal "ArgumentError", StdlibHardeningTests.raised { Base64.strict_decode64(s) }, s.inspect
      end
      assert_equal "a", Base64.strict_decode64("YQ==")
      assert_equal "", Base64.strict_decode64("")
    end

    def test_urlsafe_decode_rejects_newlines_but_takes_missing_padding
      assert_equal "ArgumentError", StdlibHardeningTests.raised { Base64.urlsafe_decode64("YQ==\n") }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { Base64.urlsafe_decode64("YQ\n") }
      assert_equal "a", Base64.urlsafe_decode64("YQ")
      assert_equal "a", Base64.urlsafe_decode64("YQ==")
    end

    def test_lenient_decode_skips_newlines
      assert_equal "a", Base64.decode64("YQ==\n")
      assert_equal "a", Base64.decode64("YQ")
      assert_equal "ab", Base64.decode64("YWI=\n")
    end
  end

  class TempfileTest < Minitest::Test
    def test_tempfile_is_private_named_by_basename_and_round_trips
      f = Tempfile.new("hard")
      path = f.path.to_s
      assert_equal true, File.basename(path).start_with?("hard")
      assert_equal 0o600, File.stat(path).mode & 0o777
      f.write("hello")
      f.close
      assert_equal "hello", File.read(path)
      assert_equal true, File.exist?(path)
      f.unlink
      assert_equal false, File.exist?(path)
      assert_nil f.path
    end

    def test_tempfile_basename_and_suffix_pair
      f = Tempfile.new(["pre", ".txt"])
      b = File.basename(f.path.to_s)
      assert_equal true, b.start_with?("pre") && b.end_with?(".txt")
      f.close
      f.unlink
    end

    def test_tempfile_create_removes_the_file_after_the_block
      seen = ""
      mode = 0
      Tempfile.create("hardc") do |g|
        seen = g.path
        mode = File.stat(seen).mode & 0o777
        g.write("x")
      end
      assert_equal 0o600, mode
      assert_equal false, File.exist?(seen)
    end

    # the failing path names the tmpdir argument, not the system temp dir
    def test_mktmpdir_error_names_the_requested_directory
      msg = ""
      begin
        Dir.mktmpdir("x", "/nonexistent/hardening") {}
      rescue SystemCallError => e
        msg = e.message
      end
      assert_equal true, msg.start_with?("No such file or directory @ dir_s_mkdir - /nonexistent/hardening/x"), msg
    end
  end

  class StringScannerTest < Minitest::Test
    # the anchored twin lives on the Regexp: a tokenizer using one built at run time, and a literal reused across scanners, both work
    def test_dynamic_regexps_scan_anchored
      out = [] #: Array[String]
      %w[foo bar baz].each do |w|
        s = StringScanner.new("#{w} 12 #{w}")
        re = Regexp.new(w)
        out << s.scan(re).to_s
        out << s.scan(re).to_s
        out << s.skip(/\s+/).to_s
        out << s.scan(/\d+/).to_s
        out << s.skip(/\s+/).to_s
        out << s.scan(re).to_s
      end
      assert_equal %w[foo 1 12 1 foo bar 1 12 1 bar baz 1 12 1 baz], out.reject(&:empty?)
    end

    def test_shared_regexp_across_scanners
      re = /\d+/
      a = StringScanner.new("12ab")
      b = StringScanner.new("ab34")
      assert_equal "12", a.scan(re)
      assert_nil b.scan(re)
      assert_equal "ab", b.scan(/[a-z]+/)
      assert_equal "34", b.scan(re)
      assert_nil a.scan(re)
    end
  end

  class JsonTest < Minitest::Test
    def test_dump_writes_to_the_io_and_returns_it
      io = StringIO.new
      ret = JSON.dump({ "a" => [1, nil] }, io)
      assert_same io, ret
      assert_equal "{\"a\":[1,null]}", io.string
    end

    def test_dump_with_nil_or_options_is_the_string
      assert_equal "[1]", JSON.dump([1], nil)
      assert_equal "[1]", JSON.dump([1])
    end

    def test_dump_allows_nan_by_default
      assert_equal "[NaN,Infinity]", JSON.dump([Float::NAN, Float::INFINITY])
      assert_equal "GeneratorError", StdlibHardeningTests.raised { JSON.dump([Float::NAN], { allow_nan: false }) }.sub("JSON::", "")
    end

    def test_parse_freeze
      doc = JSON.parse("{\"a\":[1,\"x\",{\"b\":null}]}", freeze: true)
      assert_equal true, doc.frozen?
      inner = doc["a"] #: untyped
      assert_equal true, inner.frozen?
      assert_equal true, inner[2].frozen?
      assert_equal false, JSON.parse("[1]").frozen?
      assert_equal false, JSON.parse("[1]", freeze: false).frozen?
    end

    def test_parse_option_values_that_change_nothing
      assert_equal [1], JSON.parse("[1]", allow_nan: false)
      assert_equal [1], JSON.parse("[1]", create_additions: false)
      assert_equal [1], JSON.parse("[1]", unknown_option: 1)
    end
  end
end
