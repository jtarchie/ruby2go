# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# What the methods below did, in order; tests read it to see `ensure` run.
LOG = [] #: Array[String]

class ValidationError < StandardError
  attr_reader :field #: String

  #: (String, String) -> void
  def initialize(field, msg)
    super(msg)
    @field = field
  end
end

#: (Integer) -> Integer
def checked_div(n)
  100 / n
rescue ZeroDivisionError => e
  LOG << "rescued: #{e.message}"
  -1
ensure
  LOG << "checked #{n}"
end

#: (String) -> String
def validate(s)
  raise ValidationError.new("name", "too short") if s.size < 3
  s
end

#: (Integer) -> String
def classify(n)
  begin
    raise ArgumentError, "negative" if n < 0
    return "zero" if n == 0
    "positive"
  rescue ArgumentError => e
    return "error(#{e.message})"
  ensure
    LOG << "classified #{n}"
  end
end

#: () { () -> void } -> void
def risky
  yield
rescue StandardError => e
  LOG << "caught #{e.message}"
end

class ExceptionsFlowTest < Minitest::Test
  #: () -> void
  def setup
    LOG.clear
  end

  # A method-level `rescue` supplies the value; `ensure` runs either way.
  #: () -> void
  def test_method_rescue_and_ensure
    assert_equal [20, -1], [checked_div(5), checked_div(0)]
    assert_equal ["checked 5", "rescued: divided by 0", "checked 0"], LOG
  end

  # A custom exception carries extra fields.
  #: () -> void
  def test_custom_exception_fields
    got = begin
      validate("ab")
    rescue ValidationError => e
      "#{e.field}: #{e.message}"
    end
    assert_equal "name: too short", got
    assert_equal "abc", validate("abc")
  end

  # `return` inside begin/rescue still runs `ensure` first.
  #: () -> void
  def test_return_through_ensure
    assert_equal ["error(negative)", "zero", "positive"], [classify(-1), classify(0), classify(1)]
    assert_equal ["classified -1", "classified 0", "classified 1"], LOG
  end

  #: () -> void
  def test_core_error_message
    e = assert_raises(IndexError) { [1, 2].fetch(5) }
    assert_equal "index 5 outside of array bounds: -2...2", e.message
  end

  # Raising from a `rescue` replaces the exception; bare `rescue` catches StandardError.
  #: () -> void
  def test_raise_from_rescue
    msg = ""
    begin
      begin
        raise "inner"
      rescue RuntimeError => e
        raise ArgumentError, "outer from #{e.message}"
      end
    rescue => e
      msg = e.inspect
    end
    assert_equal "#<ArgumentError: outer from inner>", msg
  end

  # An exception raised in a block is rescued by the yielding method.
  #: () -> void
  def test_rescue_around_yield
    risky { raise "in block" }
    risky { LOG << "no error" }
    assert_equal ["caught in block", "no error"], LOG
  end
end
