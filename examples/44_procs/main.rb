# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# An event bus: handlers are Procs stored in a Hash of Arrays.
class Bus
  #: () -> void
  def initialize
    @handlers = {} #: Hash[Symbol, Array[^(String) -> void]]
  end

  #: (Symbol, ^(String) -> void) -> void
  def on(event, handler)
    list = @handlers[event] || []
    list << handler
    @handlers[event] = list
  end

  #: (Symbol, String) -> Integer
  def emit(event, payload)
    list = @handlers[event] || []
    list.each { |h| h.call(payload) }
    list.size
  end
end

# Closures capture locals; lambdas compose.
#: (Integer) -> ^(Integer) -> Integer
def multiplier(n) = ->(x) { x * n }

class ProcsTest < Minitest::Test
  def test_bus_calls_stored_handlers_in_order
    log = [] #: Array[String]
    bus = Bus.new
    bus.on(:save, ->(p) { log << "saved #{p}" })
    bus.on(:save, ->(p) { log << "audit #{p.upcase}" })
    assert_equal 2, bus.emit(:save, "doc1")
    assert_equal 0, bus.emit(:delete, "doc2")
    assert_equal ["saved doc1", "audit DOC1"], log
  end

  def test_lambdas_call_and_compose
    triple = multiplier(3)
    succ = ->(x) { x + 1 } #: ^(Integer) -> Integer
    assert_equal 15, triple.call(5)
    assert_equal 18, triple.(6)
    assert_equal 21, triple[7]
    assert_equal 7, (triple >> succ).call(2)
    assert_equal 9, (triple << succ).call(2)
    assert_equal [3, 6, 9], [1, 2, 3].map(&triple)

    pipeline = [succ, triple, succ] #: Array[^(Integer) -> Integer]
    assert_equal 16, pipeline.reduce(4) { |acc, f| f.call(acc) }
  end

  def test_closures_capture_locals
    count = 0
    tick = -> { count += 1 }
    5.times { tick.call }
    assert_equal 5, count
  end

  # `return` leaves a lambda; `next` does too.
  def test_return_and_next_inside_a_lambda
    clamp = lambda do |x|
      return 0 if x < 0
      next 100 if x > 100
      x
    end #: ^(Integer) -> Integer
    assert_equal [0, 50, 100], [-5, 50, 500].map(&clamp)
  end

  def test_proc_passed_as_a_block
    out = [] #: Array[String]
    shout = proc { |s| out << s.upcase + "!" } #: ^(String) -> void
    %w[hey you].each(&shout)
    assert_equal ["HEY!", "YOU!"], out
  end

  def test_proc_reflection
    triple = multiplier(3)
    assert_equal 1, triple.arity
    assert_equal true, triple.lambda?
    assert_equal Proc, triple.class
    assert_equal true, triple.is_a?(Proc)
  end
end
