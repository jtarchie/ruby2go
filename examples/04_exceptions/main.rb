# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class AppError < StandardError; end
class NotFound < AppError; end

#: (Hash[String, Integer], String) -> Integer
def lookup(h, k)
  h.fetch(k)
rescue KeyError
  raise NotFound, "missing #{k}"
end

class ExceptionsTest < Minitest::Test
  # `rescue AppError` catches the NotFound subclass; `ensure` runs after it.
  #: () -> void
  def test_rescue_superclass_then_ensure
    log = [] #: Array[String]
    begin
      lookup({ "a" => 1 }, "b")
    rescue AppError => e
      log << "handled: #{e.message}"
    ensure
      log << "done"
    end
    assert_equal ["handled: missing b", "done"], log
  end

  #: () -> void
  def test_rescue_and_reraise
    e = assert_raises(NotFound) { lookup({ "a" => 1 }, "b") }
    assert_equal "missing b", e.message
    assert_equal 1, lookup({ "a" => 1 }, "a")
  end
end
