require "minitest/autorun"

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
