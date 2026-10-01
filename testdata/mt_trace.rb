# frozen_string_literal: true

# The trace oracle's MRI half (decision 105): `ruby -r ./testdata/mt_trace.rb
# x_test.rb` with RB2GO_MT_TRACE=<path> appends one line per assertion a
# test calls, "Class#test assertion operand...", operands inspected (minitest's
# mu_pp minus its encoding note: rb2go's strings carry no encoding), the
# lines rb2go's prelude/minitest.rb writes. Assertions an assertion calls
# itself (assert_empty's assert_respond_to) are not logged: a depth counter.
require "minitest"

module RB2GoMtTrace
  # Each assertion's operands (the message argument is not one), with the
  # defaults MRI fills in, so a call that omits `delta` logs what rb2go sees.
  UNDEFINED = Minitest::Assertions::UNDEFINED
  OPERANDS = {
    assert: [nil], assert_empty: [nil], assert_equal: [nil, nil],
    assert_in_delta: [nil, nil, 0.001], assert_in_epsilon: [nil, nil, 0.001],
    assert_includes: [nil, nil], assert_instance_of: [nil, nil], assert_kind_of: [nil, nil],
    assert_match: [nil, nil], assert_nil: [nil], assert_operator: [nil, nil, UNDEFINED],
    assert_path_exists: [nil], assert_predicate: [nil, nil], assert_respond_to: [nil, nil],
    assert_same: [nil, nil], flunk: [nil], pass: [],
    refute: [nil], refute_empty: [nil], refute_equal: [nil, nil],
    refute_in_delta: [nil, nil, 0.001], refute_in_epsilon: [nil, nil, 0.001],
    refute_includes: [nil, nil], refute_instance_of: [nil, nil], refute_kind_of: [nil, nil],
    refute_match: [nil, nil], refute_nil: [nil], refute_operator: [nil, nil, UNDEFINED],
    refute_path_exists: [nil], refute_predicate: [nil, nil], refute_respond_to: [nil, nil],
    refute_same: [nil, nil],
  }.freeze

  FILE = File.open(ENV.fetch("RB2GO_MT_TRACE"), "a")
  FILE.sync = true

  @depth = 0
  class << self
    attr_accessor :depth
  end

  def self.log(test, kind, operands)
    FILE.puts("#{test.class.name}##{test.name} #{kind} #{operands.map(&:inspect).join(" ")}")
  end

  # Called on the way in: logs an outermost assertion, then nests.
  def self.enter(test, kind, args)
    if depth.zero?
      defaults = OPERANDS.fetch(kind)
      operands = defaults.each_with_index.map { |d, i| i < args.size ? args[i] : d }
      logged = kind
      if operands.last.equal?(UNDEFINED) # assert_operator a, :pred is assert_predicate, which it delegates to
        logged = kind == :assert_operator ? :assert_predicate : :refute_predicate
        operands.pop
      end
      log(test, logged, operands)
    end
    self.depth += 1
  end

  def self.leave
    self.depth -= 1
  end

  # Real methods, not define_method blocks: minitest's Assertion#location
  # takes the frame after the last one named like an assertion, so these
  # frames must read 'RB2GoMtTrace#assert_x' too.
  OPERANDS.each_key do |kind|
    module_eval(<<~RUBY, __FILE__, __LINE__ + 1)
      def #{kind}(*args, &blk)
        RB2GoMtTrace.enter(self, :#{kind}, args)
        super
      ensure
        RB2GoMtTrace.leave
      end
    RUBY
  end

  # Logged after the block: the classes expected, then the class and message of what came.
  def assert_raises(*exp, &blk)
    outer = RB2GoMtTrace.depth.zero?
    RB2GoMtTrace.depth += 1
    begin
      e = super
    ensure
      RB2GoMtTrace.depth -= 1
    end
    if outer
      classes = exp.dup
      classes.pop if classes.last.is_a?(String)
      classes << StandardError if classes.empty?
      RB2GoMtTrace.log(self, :assert_raises, [*classes, e.class, e.message])
    end
    e
  end
end

Minitest::Assertions.prepend(RB2GoMtTrace)
