require "minitest/autorun"
require "English"
require "stringio"

$globals_count = 0
$globals_names = [] #: Array[String]
$globals_label = nil #: String?

def globals_bump(n)
  $globals_count += n
  $globals_names << "b#{n}"
end

# never called: a literal past 64 bits compiles, raising only when run (decision 361)
def globals_big = 0x1_0000_0000_0000_0000

module GlobalsTests
  class Opts
    def pick(cls = nil, msg = nil)
      return "none" if msg.nil?

      msg.is_a?(Regexp) ? "re" : "#{cls}:#{msg}"
    end
  end

  class GlobalsTest < Minitest::Test
    def test_program_globals
      globals_bump(2)
      globals_bump(3)
      assert_equal 5, $globals_count
      assert_equal %w[b2 b3], $globals_names
      $globals_label ||= "first"
      $globals_label ||= "second"
      assert_equal "first", $globals_label
      $globals_flag = true
      $globals_flag &&= $globals_count > 9
      assert_equal false, $globals_flag
    end

    def test_only_nil_local
      x = nil
      assert_nil x
      assert_equal "nil", x.inspect
      assert_equal [], x.to_a
      r = (1 if x)
      assert_nil r
    end

    def test_param_typed_only_by_nil_default
      assert_equal "none", Opts.new.pick
    end
  end
end

# English's names and the separators they alias (decision 415): $; for split, $, for join, $\\ after print.
module GlobalsTests
  class EnglishTest < Minitest::Test
    def test_match_references
      "hello world" =~ /o (w)(x)?/
      assert_equal(["o w", "hell", "orld", "w"], [$MATCH, $PREMATCH, $POSTMATCH, $LAST_PAREN_MATCH])
      assert_equal([$`, $'], [$PREMATCH, $POSTMATCH])
    end

    def test_error_info_and_position
      begin
        raise "boom"
      rescue
        assert_equal("boom", $ERROR_INFO.message)
        assert($ERROR_POSITION.is_a?(Array))
      end
      assert_nil($ERROR_INFO)
      assert((1 / 0 rescue $ERROR_POSITION).is_a?(Array))
    end

    def test_streams_and_argv
      assert($DEFAULT_OUTPUT == $stdout)
      assert($ARGV == ARGV)
    end

    def test_field_separator_splits
      $; = ","
      assert_equal(["x", "y z"], "x,y z".split)
      assert_equal(",", $FS)
    ensure
      $; = nil
    end

    def test_output_field_separator_joins
      $, = "-"
      assert_equal("1-2", [1, 2].join)
      assert_equal("1+2", [1, 2].join("+"))
      assert_equal("-", $OFS)
    ensure
      $, = nil
    end

    def test_print_separators
      out = StringIO.new
      $, = ","
      $\ = "!"
      out.print("a", "b")
      assert_equal("a,b!", out.string)
      assert_equal("!", $ORS)
    ensure
      $, = nil
      $\ = nil
    end
  end
end
