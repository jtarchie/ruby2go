# rbs_inline: enabled
# A port of minitest 6.0.6 (MRI 4.0's bundled one), MIT licensed: see prelude/minitest.LICENSE. Shape changes are marked "port:".

module Kernel
  private

  #: () -> void
  def __require_minitest_autorun = Minitest.autorun

  # Never called: a computed send in the closed world notes every method name for Dyn wrappers; rbMtCall's _Call tables then take the ones user classes define (callableNames).
  # @dynamic
  #: (untyped, untyped) -> untyped
  def __mt_send_probe(recv, name) = recv.__send__(name)
end

module Minitest
  VERSION = "6.0.6" #: String

  # port: @@installed_at_exit, as a flag nothing can reset.
  AUTORUN = [] #: Array[bool]

  # port: OptionParser's help text, verbatim.
  HELP = <<~HELP #: String
    Usage: minitest [paths]     [options]
       or: ruby path/to/test.rb [options]
       or: rake test          [A=options] (see Minitest::TestTask for more options)

        -h, --help                       Display this help.
        -V, --version                    Display the version.
        -s, --seed SEED                  Sets random seed. Also via env, eg: SEED=42
        -v, --verbose                    Verbose. Print each name as they run.
        -q, --quiet                      Quiet. Show no dots while processing files.
            --show-skips                 Show skipped at the end of run.
        -i, --include PATTERN            Include /regexp/ or string for run.
        -e, --exclude PATTERN            Exclude /regexp/ or string from run.
        -S, --skip CODES                 Skip reporting of certain types of results (eg E).
        -W[error]                        Turn Ruby warnings into errors
  HELP

  # port: cattr_accessor :seed, in a constant because a module's ivars can't be optional yet.
  SEED = [0] #: Array[Integer]

  #: () -> Integer
  def self.seed = SEED.first.to_i

  #: (Integer) -> void
  def self.seed=(s)
    SEED[0] = s
  end

  #: () -> void
  def self.autorun
    return unless AUTORUN.empty?

    AUTORUN << true
    at_exit do
      # port: no `$!`; a failing exit status so far means the program already failed.
      next unless __exit_status_ok?

      exit_code = false
      at_exit do
        Minitest.__run_after_run
        exit(exit_code ? 0 : 1)
      end
      exit_code = Minitest.run(ARGV)
    end
  end

  # port: @@after_run lives in Go, since a block can't be stored from Ruby.
  #: () { () -> void } -> void
  def self.after_run = %x{ rbMtAfterRun = append(rbMtAfterRun, blk) }

  #: () -> void
  def self.__run_after_run = %x{ for i := len(rbMtAfterRun) - 1; i >= 0; i-- { rbMtAfterRun[i]() } }

  #: (Array[String]) -> Options
  def self.process_args(args = [])
    options = Options.new
    orig_args = args.dup
    rest = [] #: Array[String]
    i = 0
    while i < args.size
      a = args.fetch(i)
      i += 1
      # port: hand-parsed instead of OptionParser; long options also take `--opt=value`.
      name, eq, inline = a.partition("=")
      value = eq.empty? ? nil : inline
      case eq.empty? || !name.start_with?("--") ? a : name
      when "-h", "--help"
        STDOUT.puts(HELP)
        __exit_bang(0)
      when "-V", "--version"
        STDOUT.puts("minitest #{VERSION}")
        __exit_bang(0)
      when "-s", "--seed", "-i", "--include", "-n", "--name", "-e", "--exclude", "-x", "-S", "--skip"
        if value.nil?
          value = args[i]
          i += 1
        end
        raise ArgumentError, "missing argument: #{a}" unless value

        v = value.to_s
        case name
        when "-s", "--seed" then options.seed = v.to_i
        when "-i", "--include", "-n", "--name" then options.include = v
        when "-e", "--exclude", "-x" then options.exclude = v
        else options.skip = v.chars
        end
      when "-v", "--verbose" then options.verbose = true
      when "-q", "--quiet" then options.quiet = true
      when "--show-skips" then options.show_skips = true
      else
        if a.start_with?("-s") && a.size > 2
          options.seed = (a[2..] || "").to_i
        elsif a.start_with?("-W")
          nil # port: warnings aren't errors
        elsif a.start_with?("-") && a != "-"
          STDOUT.puts
          STDOUT.puts("invalid option: #{a}")
          STDOUT.puts
          STDOUT.puts(HELP)
          exit 1
        else
          rest << a
        end
      end
    end
    orig_args -= rest

    unless options.seed
      srand
      options.seed = (ENV["SEED"] || srand.to_s).to_i % 0xFFFF
      orig_args << "--seed" << options.seed.to_s
    end

    options.args = orig_args.map { |s| s.match?(/[\s|&<>$()]/) ? s.inspect : s }.join(" ")
    options
  end

  #: (?Array[String]) -> bool
  def self.run(args = [])
    options = process_args(args)

    Minitest.seed = options.seed || 0
    srand(Minitest.seed)

    reporter = CompositeReporter.new
    summary = SummaryReporter.new(STDOUT, options)
    reporter << summary
    reporter << ProgressReporter.new(STDOUT, options) unless options.quiet

    reporter.start
    run_all_suites(reporter, options)

    reporter.report

    return empty_run!(options) if summary.count == 0

    reporter.passed?
  end

  #: (Options) -> bool
  def self.empty_run!(options)
    filter = options.include
    return true unless filter # no filter, but nothing ran == success

    STDERR.puts("Nothing ran for filter: %s" % [filter])
    # port: no DidYouMean suggestions.
    false
  end

  #: (CompositeReporter, Options) -> void
  def self.run_all_suites(reporter, options)
    # port: no parallel executor, so every suite is serial.
    Runnable.runnables.shuffle.each { |suite| suite.run_suite(reporter, options) }
  end

  #: () -> Float
  def self.clock_time = Process.clock_gettime(Process::CLOCK_MONOTONIC)

  # port: BacktraceFilter#filter. MRI drops its own lib/minitest frames; here the runner's frames carry Minitest:: labels (decision 106).
  #: (Array[String]?) -> Array[String]
  def self.filter_backtrace(bt)
    return ["No backtrace"] unless bt

    new_bt = bt.take_while { |line| !line.include?("in 'Minitest::") }
    new_bt = bt.reject { |line| line.include?("in 'Minitest::") } if new_bt.empty?
    new_bt = bt.dup if new_bt.empty?
    new_bt
  end

  # port: the options hash, typed.
  class Options < Object
    attr_accessor :seed #: Integer?
    attr_accessor :verbose #: bool
    attr_accessor :quiet #: bool
    attr_accessor :show_skips #: bool
    attr_accessor :include #: String?
    attr_accessor :exclude #: String?
    attr_accessor :skip #: Array[String]
    attr_accessor :args #: String

    #: () -> void
    def initialize
      @seed = nil
      @verbose = false
      @quiet = false
      @show_skips = false
      @include = nil
      @exclude = nil
      @skip = []
      @args = ""
    end
  end

  class Runnable < Object
    attr_accessor :assertions #: Integer

    attr_accessor :failures #: Array[Minitest::Assertion]

    attr_accessor :time #: Float

    #: () { () -> void } -> void
    def time_it
      t0 = Minitest.clock_time
      begin
        yield
      ensure
        self.time = Minitest.clock_time - t0
      end
    end

    #: () -> String
    def name = @NAME

    #: (String) -> void
    def name=(o)
      @NAME = o
    end

    #: (Regexp) -> Array[String]
    def self.methods_matching(re) = public_instance_methods(true).map(&:to_s).select { |m| re.match?(m) } # port: not grep, whose untyped `pattern === x` keeps === on every class

    #: (Options) -> Array[String]
    def self.filter_runnable_methods(options)
      pos = __filter(options.include, true)
      neg = __filter(options.exclude, false)
      full = "#{self}#"
      runnable_methods
        .select { |m| pos.call(m) || pos.call(full + m) }
        .reject { |m| neg.call(m) || neg.call(full + m) }
    end

    # port: `pos === m`, where a "/re/" string became a Regexp; no pattern answers +absent+.
    #: (String?, bool) -> ^(String) -> bool
    def self.__filter(pat, absent)
      if pat.nil?
        none = ->(_s) { absent } #: ^(String) -> bool
        return none
      end
      md = pat.match(%r{/(.*)/})
      if md
        re = /#{md[1]}/
        by_re = ->(s) { re.match?(s) } #: ^(String) -> bool
        return by_re
      end
      by_name = ->(s) { s == pat } #: ^(String) -> bool
      by_name
    end

    #: (AbstractReporter, Options) -> void
    def self.run_suite(reporter, options)
      filtered_methods = filter_runnable_methods(options)
      return if filtered_methods.empty?

      filtered_methods.each do |method_name|
        run_one(method_name, reporter)
      end
    end

    # port: Runnable.run(klass, method_name, reporter), on the class itself.
    #: (String, AbstractReporter) -> void
    def self.run_one(method_name, reporter)
      reporter.prerecord(self, method_name)
      reporter.record(new(method_name).run)
    end

    #: () -> Symbol
    def self.run_order = :random

    #: () -> Array[String]
    def self.runnable_methods = raise(NotImplementedError, "subclass responsibility")

    # port: no inherited hook filling @@runnables while the program runs: the compiler lists every descendant in definition order (decision 79), which is the order the hook would have seen. Result is left out: MRI defines the hook after it.
    #: () -> Array[singleton(Minitest::Runnable)]
    def self.runnables = %x{
      out := &Array[Minitest_Runnable_MetaI]{}
      for _, k := range rbDescendants(Minitest_Runnable_class) {
        if k != any(Minitest_Result_class) {
          *out = append(*out, k.(Minitest_Runnable_MetaI))
        }
      }
      return out
    }

    # port: no `failure` (failures.first): a `Minitest::Assertion?` box would reach the pruner's rbUnbox in every program.

    #: (String) -> void
    def initialize(name)
      @NAME = name
      @failures = [] #: Array[Minitest::Assertion]
      @assertions = 0
      @time = 0.0
    end

    # port: Reportable's methods, here because a module can't see its includers' attribute types.
    #: () -> bool
    def passed? = failures.empty?

    #: () -> String
    def location
      loc = " [#{failures.fetch(0).location.delete_prefix("#{Dir.pwd}/")}]" unless passed? || error?
      "#{class_name}##{name}#{loc}"
    end

    #: () -> String
    def result_code
      passed? ? "." : failures.fetch(0).result_code
    end

    #: () -> bool
    def skipped? = !passed? && failures.fetch(0).is_a?(Skip)

    #: () -> bool
    def error? = failures.any? { |f| f.is_a?(UnexpectedError) }

    #: () -> String
    def class_name = raise(NotImplementedError, "subclass responsibility")

    #: () -> Minitest::Result
    def run = raise(NotImplementedError, "subclass responsibility")
  end

  class Result < Runnable
    attr_accessor :klass #: String

    #: (Minitest::Runnable) -> Minitest::Result
    def self.from(runnable)
      r = new(runnable.name)
      r.klass = runnable.class.name
      r.assertions = runnable.assertions
      r.failures = runnable.failures.dup
      r.time = runnable.time
      r
    end

    #: (String) -> void
    def initialize(name)
      super
      @klass = ""
    end

    #: () -> String
    def class_name = klass

    #: () -> String
    def to_s
      return location if passed? && !skipped?

      failures.map { |failure| "#{failure.result_label}:\n#{location}:\n#{failure.message}\n" }.join("\n")
    end
  end

  class AbstractReporter < Object
    #: () -> void
    def start = nil

    #: (singleton(Minitest::Runnable), String) -> void
    def prerecord(klass, name) = nil

    #: (Minitest::Result) -> void
    def record(result) = nil

    #: () -> void
    def report = nil

    #: () -> bool
    def passed? = true
  end

  class Reporter < AbstractReporter
    attr_accessor :io #: IO

    attr_accessor :options #: Options

    #: (IO, Options) -> void
    def initialize(io, options)
      @io = io
      @options = options
    end
  end

  class ProgressReporter < Reporter
    #: (singleton(Minitest::Runnable), String) -> void
    def prerecord(klass, name)
      return unless options.verbose

      io.print("%s#%s = " % [klass.name, name])
      io.flush
    end

    #: (Minitest::Result) -> void
    def record(result)
      io.print("%.2f s = " % [result.time]) if options.verbose
      io.print(result.result_code)
      io.puts if options.verbose
    end
  end

  class StatisticsReporter < Reporter
    attr_accessor :assertions #: Integer
    attr_accessor :count #: Integer
    attr_accessor :results #: Array[Minitest::Result]
    attr_accessor :start_time #: Float
    attr_accessor :total_time #: Float
    attr_accessor :failures #: Integer
    attr_accessor :errors #: Integer
    attr_accessor :skips #: Integer

    #: (IO, Options) -> void
    def initialize(io, options)
      super
      @assertions = 0
      @count = 0
      @results = [] #: Array[Minitest::Result]
      @start_time = 0.0
      @total_time = 0.0
      @failures = 0
      @errors = 0
      @skips = 0
    end

    #: () -> bool
    def passed? = results.all?(&:skipped?)

    #: () -> void
    def start
      self.start_time = Minitest.clock_time
    end

    #: (Minitest::Result) -> void
    def record(result)
      self.count += 1
      self.assertions += result.assertions
      results << result if !result.passed? || result.skipped?
    end

    # port: counted by result_label rather than grouped by failure class.
    #: () -> void
    def report
      self.total_time = Minitest.clock_time - start_time
      labels = results.map { |r| r.passed? ? "" : r.failures.fetch(0).result_label }
      self.failures = labels.count("Failure")
      self.errors = labels.count("Error")
      self.skips = labels.count("Skipped")
    end
  end

  class SummaryReporter < StatisticsReporter
    # port: no io.sync; stdout is flushed at exit.
    #: () -> void
    def start
      super
      io.puts("Run options: #{options.args}")
      io.puts
      io.puts("# Running:")
      io.puts
    end

    #: () -> void
    def report
      super
      io.puts unless options.verbose # finish the dots
      io.puts
      io.puts(statistics)
      aggregated_results(io)
      io.puts(summary)
    end

    #: () -> String
    def statistics
      "Finished in %.6fs, %.4f runs/s, %.4f assertions/s." % [total_time, count / total_time, assertions / total_time]
    end

    #: (IO) -> void
    def aggregated_results(io)
      filtered_results = results.dup
      filtered_results.reject!(&:skipped?) unless options.verbose || options.show_skips

      skip = options.skip
      filtered_results.each_with_index do |result, i|
        next if skip.include?(result.result_code)

        io.puts("\n%3d) %s" % [i + 1, result.to_s])
      end
      io.puts
    end

    #: () -> String
    def summary
      extra = ""
      if results.any?(&:skipped?) && !(options.verbose || options.show_skips || ENV["MT_NO_SKIP_MSG"])
        extra = "\n\nYou have skipped tests. Run with --verbose for details."
      end
      "%d runs, %d assertions, %d failures, %d errors, %d skips%s" % [count, assertions, failures, errors, skips, extra]
    end
  end

  class CompositeReporter < AbstractReporter
    attr_accessor :reporters #: Array[Minitest::AbstractReporter]

    #: () -> void
    def initialize
      @reporters = [] #: Array[Minitest::AbstractReporter]
    end

    #: (Minitest::AbstractReporter) -> void
    def <<(reporter)
      reporters << reporter
    end

    #: () -> bool
    def passed? = reporters.all?(&:passed?)

    #: () -> void
    def start = reporters.each(&:start)

    #: (singleton(Minitest::Runnable), String) -> void
    def prerecord(klass, name)
      reporters.each { |reporter| reporter.prerecord(klass, name) }
    end

    #: (Minitest::Result) -> void
    def record(result)
      reporters.each { |reporter| reporter.record(result) }
    end

    #: () -> void
    def report = reporters.each(&:report)
  end

  class Assertion < Exception
    # port: rb2go has no backtraces, so the failure's file:line is found when it's made (rbMtLocation).
    #: (?String?) -> void
    def initialize(msg = nil)
      super
      @location = __mt_location
    end

    #: () -> Exception
    def error = self

    #: () -> String
    def location = @location

    #: () -> String
    def result_code = result_label[0, 1] || ""

    #: () -> String
    def result_label = "Failure"

    #: () -> String
    def self.__mt_location = %x{ return rbMtLocation() }

    #: () -> String
    def __mt_location = Minitest::Assertion.__mt_location
  end

  class Skip < Assertion
    #: () -> String
    def result_label = "Skipped"
  end

  class UnexpectedError < Assertion
    attr_accessor :error #: Exception

    #: (Exception) -> void
    def initialize(error)
      super("Unexpected exception")
      @error = error
    end

    #: () -> String
    def message = "#{error.class.name}: #{error.message}\n    #{Minitest.filter_backtrace(error.backtrace).join("\n    ")}"

    #: () -> String
    def result_label = "Error"
  end

  module Guard
    #: () -> bool
    def jruby? = false

    #: () -> bool
    def mri? = true

    #: () -> bool
    def osx? = %x{ Boolean(runtime.GOOS == "darwin") }

    #: () -> bool
    def windows? = %x{ Boolean(runtime.GOOS == "windows") }
  end

  # port: Assertions::UNDEFINED, assert_operator's "no o2" sentinel.
  class Undefined < Object
    #: () -> String
    def inspect = "UNDEFINED"
  end

  # port: only the UNDEFINED sentinel; the assertions are Test's own methods (a module can't see Test's assertion counter's type).
  module Assertions
    UNDEFINED = Undefined.new #: Undefined
  end

  class Test < Runnable
    include Minitest::Guard

    #: (untyped, untyped) -> String
    def diff(exp, act)
      expect, butwas = things_to_diff(exp, act)
      return "Expected: #{mu_pp(exp)}\n  Actual: #{mu_pp(act)}" unless expect && butwas

      result = __mt_diff(expect, butwas)
      if result.empty?
        klass = __mt_class_name(exp)
        result = [
          "No visible difference in the #{klass}#inspect output.\n",
          "You should look at the implementation of #== on ",
          "#{klass} or its members.\n",
          expect,
        ].join
      end
      result
    end

    #: (untyped, untyped) -> [String?, String?]
    def things_to_diff(exp, act)
      expect = mu_pp_for_diff(exp)
      butwas = mu_pp_for_diff(act)

      e1 = expect.include?("\n")
      e2 = expect.include?("\\n")
      b1 = butwas.include?("\n")
      b2 = butwas.include?("\\n")

      need_to_diff = (e1 ^ e2 || b1 ^ b2 || expect.size > 30 || butwas.size > 30 || expect == butwas) && __mt_have_diff
      need_to_diff ? [expect, butwas] : [nil, nil]
    end

    # @dynamic
    #: (untyped) -> String
    def mu_pp(obj) = obj.inspect

    #: (untyped) -> String
    def mu_pp_for_diff(obj) = __mt_pp_for_diff(mu_pp(obj))

    #: (String) -> String
    def __mt_pp_for_diff(str) = %x{ return rbMtPPForDiff(str) }

    # The trace oracle (decision 105): with RB2GO_MT_TRACE set, each assertion a test calls logs
    # one line, "Class#test assertion operand...", operands inspected. Only the outermost
    # assertion logs: the ones an assertion calls itself go through __mt_* twins that don't,
    # and assert_raises's block runs nested (__mt_trace_nest), as MRI's depth counter sees it.
    #: (String, *untyped) -> void
    def __mt_trace(kind, *operands)
      return unless __mt_trace_on?

      __mt_trace_line("#{__mt_class_name(self)}##{name} #{kind} #{operands.map { |o| __mt_pp(o) }.join(" ")}")
    end

    #: () -> bool
    def __mt_trace_on? = %x{ return Boolean(rbMtTraceOn()) }

    #: (Integer) -> void
    def __mt_trace_nest(by) = %x{ rbMtTraceDepth += int(by) }

    #: (String) -> void
    def __mt_trace_line(line) = %x{ rbMtTrace(line) }

    #: (untyped, ?untyped) -> bool
    def assert(test, msg = nil)
      __mt_trace("assert", test)
      return __assert(test, -> { "Expected #{mu_pp(test)} to be truthy." }) if msg.nil?

      __assert(test, -> { __s(msg) })
    end

    # port: every assertion ends here and counts one, as minitest's `assert` does.
    #: (untyped, ^() -> String) -> bool
    def __assert(test, msg)
      self.assertions += 1
      raise Minitest::Assertion, msg.call unless test

      true
    end

    # @dynamic
    #: (untyped, ?untyped) -> bool
    def assert_empty(obj, msg = nil)
      __mt_trace("assert_empty", obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to be empty" })
      __mt_respond_to(obj, :empty?)
      __assert(obj.empty?, m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def assert_equal(exp, act, msg = nil)
      __mt_trace("assert_equal", exp, act)
      m = message(msg, nil, -> { diff(exp, act) })
      __assert(false, message(nil, ".", -> { "Use assert_nil if expecting nil" })) if exp.nil? # refute_nil with a proc message
      __assert(exp == act, m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped, ?untyped) -> bool
    def assert_in_delta(exp, act, delta = 0.001, msg = nil)
      __mt_trace("assert_in_delta", exp, act, delta)
      __mt_in_delta(exp, act, delta, msg, false)
    end

    # port: assert_in_delta's body, shared with assert_in_epsilon and the refutes, so each logs once.
    # @dynamic
    #: (untyped, untyped, untyped, untyped, bool) -> bool
    def __mt_in_delta(exp, act, delta, msg, refute)
      n = (exp - act).abs
      m = message(msg, ".", -> { "Expected |#{__s(exp)} - #{__s(act)}| (#{__s(n)}) to #{refute ? "not " : ""}be <= #{__s(delta)}" })
      __assert(refute ? !(delta >= n) : delta >= n, m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped, ?untyped) -> bool
    def assert_in_epsilon(exp, act, epsilon = 0.001, msg = nil)
      __mt_trace("assert_in_epsilon", exp, act, epsilon)
      __mt_in_delta(exp, act, (exp.abs < act.abs ? exp.abs : act.abs) * epsilon, msg, false)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def assert_includes(collection, obj, msg = nil)
      __mt_trace("assert_includes", collection, obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(collection)} to include #{mu_pp(obj)}" })
      __mt_respond_to(collection, :include?)
      __assert(collection.include?(obj), m)
    end

    #: (untyped, untyped, ?untyped) -> bool
    def assert_instance_of(cls, obj, msg = nil)
      __mt_trace("assert_instance_of", cls, obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to be an instance of #{__s(cls)}, not #{__mt_class_name(obj)}" })
      __assert(__mt_class_name(obj) == __s(cls), m)
    end

    #: (untyped, untyped, ?untyped) -> bool
    def assert_kind_of(cls, obj, msg = nil)
      __mt_trace("assert_kind_of", cls, obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to be a kind of #{__s(cls)}, not #{__mt_class_name(obj)}" })
      __assert(obj.kind_of?(cls), m)
    end

    #: (untyped, untyped, ?untyped) -> MatchData?
    def assert_match(matcher, obj, msg = nil)
      __mt_trace("assert_match", matcher, obj)
      __mt_respond_to(matcher, :=~)
      re = __mt_regexp(matcher) # before the message, which MRI builds lazily after this conversion
      m = message(msg, ".", -> { "Expected #{mu_pp(re)} to match #{mu_pp(obj)}" })
      md = __mt_match(re, obj)
      __assert(md, m)
      md
    end

    #: (untyped, ?untyped) -> bool
    def assert_nil(obj, msg = nil)
      __mt_trace("assert_nil", obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to be nil" })
      __assert(obj.nil?, m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped, ?untyped) -> bool
    def assert_operator(o1, op, o2 = Assertions::UNDEFINED, msg = nil)
      if Assertions::UNDEFINED.equal?(o2)
        __mt_trace("assert_predicate", o1, op)
        return __mt_predicate(o1, op, msg, false)
      end

      __mt_trace("assert_operator", o1, op, o2)
      __mt_respond_to(o1, op)
      m = message(msg, ".", -> { "Expected #{mu_pp(o1)} to be #{__s(op)} #{mu_pp(o2)}" })
      __assert(__mt_send(o1, op, o2), m)
    end

    #: (String, ?untyped) -> bool
    def assert_path_exists(path, msg = nil)
      __mt_trace("assert_path_exists", path)
      m = message(msg, ".", -> { "Expected path '#{__s(path)}' to exist" })
      __assert(File.exist?(path), m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def assert_predicate(o1, op, msg = nil)
      __mt_trace("assert_predicate", o1, op)
      __mt_predicate(o1, op, msg, false)
    end

    # port: assert_predicate's body, shared with assert_operator and the refutes, so each logs once.
    # @dynamic
    #: (untyped, untyped, untyped, bool) -> bool
    def __mt_predicate(o1, op, msg, refute)
      __mt_respond_to(o1, op)
      m = message(msg, ".", -> { "Expected #{mu_pp(o1)} to #{refute ? "not " : ""}be #{__s(op)}" })
      test = __mt_send(o1, op)
      __assert(refute ? !test : test, m)
    end

    # @dynamic
    #: (*untyped) { () -> void } -> Exception
    def assert_raises(*exp)
      msg = exp.last.is_a?(String) ? "#{exp.pop}.\n" : ""
      exp << StandardError if exp.empty?

      __mt_trace_nest(1)
      begin
        yield
      rescue Exception => e
        __mt_trace_nest(-1)
        if exp.any? { |k| __mt_kind_of(k, e) }
          __mt_trace("assert_raises", *exp, e.class, e.message) # the classes expected, then what came
          __assert(true, -> { "" }) # pass: count assertion
          return e
        end
        raise e if e.is_a?(Minitest::Assertion) || e.is_a?(SignalException) || e.is_a?(SystemExit)

        __mt_flunk(exception_details(e, "#{__s(msg)}#{mu_pp(exp)} exception expected, not"))
      end
      __mt_trace_nest(-1)

      shown = exp.size == 1 ? exp.first : exp
      __mt_flunk("#{__s(msg)}#{mu_pp(shown)} expected but nothing was raised.")
      raise "unreachable: flunk raises"
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def assert_respond_to(obj, meth, msg = nil)
      __mt_trace("assert_respond_to", obj, meth)
      __mt_respond_to(obj, meth, msg)
    end

    # port: assert_respond_to's body, which other assertions call, so each logs once.
    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def __mt_respond_to(obj, meth, msg = nil)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} (#{__mt_class_name(obj)}) to respond to ##{__s(meth)}" })
      __assert(obj.respond_to?(meth), m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def assert_same(exp, act, msg = nil)
      __mt_trace("assert_same", exp, act)
      m = message(msg, ".", lambda do
        "Expected %s (oid=%d) to be the same as %s (oid=%d)" % [mu_pp(act), act.object_id, mu_pp(exp), exp.object_id]
      end)
      __assert(false, message(nil, ".", -> { "Use assert_nil if expecting nil" })) if exp.nil? # refute_nil with a proc message
      __assert(exp.equal?(act), m)
    end

    # Captures what the block writes to $stdout and $stderr (decision 109).
    #: () { () -> void } -> [String, String]
    def capture_io
      captured_stdout = StringIO.new
      captured_stderr = StringIO.new
      $stdout = captured_stdout
      $stderr = captured_stderr
      begin
        yield
      ensure
        $stdout = STDOUT
        $stderr = STDERR
      end
      [captured_stdout.string, captured_stderr.string]
    end

    # Each expectation is a String (assert_equal) or a Regexp (assert_match); nil leaves that stream unchecked.
    #: (?untyped, ?untyped) { () -> void } -> bool
    def assert_output(stdout = nil, stderr = nil, &blk)
      __mt_trace("assert_output", stdout, stderr)
      out, err = capture_io(&blk)
      y = __mt_output(stderr, err, "In stderr") if stderr
      x = __mt_output(stdout, out, "In stdout") if stdout
      (!stdout || x) && (!stderr || y)
    rescue Minitest::Assertion => e
      raise e
    rescue StandardError => e
      raise UnexpectedError.new(e)
    end

    #: () { () -> void } -> bool
    def assert_silent(&blk)
      __mt_trace("assert_silent")
      out, err = capture_io(&blk)
      y = __mt_output("", err, "In stderr")
      x = __mt_output("", out, "In stdout")
      x && y
    end

    # port: assert_output's `send out_msg, ...`: assert_match for a Regexp, assert_equal otherwise, neither logged.
    #: (untyped, String, String) -> bool
    def __mt_output(exp, act, where)
      if exp.is_a?(Regexp)
        self.assertions += 1 # assert_match's assert_respond_to(matcher, :=~)
        m = message(where, ".", -> { "Expected #{mu_pp(exp)} to match #{mu_pp(act)}" })
        return __assert(__mt_match(exp, act), m)
      end
      m = message(where, nil, -> { diff(exp, act) })
      __assert(exp == act, m)
    end

    #: (Exception, String) -> String
    def exception_details(e, msg)
      [
        msg,
        "Class: <#{e.class.name}>",
        "Message: <#{e.message.inspect}>",
        "---Backtrace---",
        Minitest.filter_backtrace(e.backtrace).join("\n"),
        "---------------",
      ].join("\n")
    end

    #: (?untyped) -> bool
    def flunk(msg = nil)
      __mt_trace("flunk", msg)
      __mt_flunk(msg)
    end

    # port: flunk's body, which assert_raises calls, so each logs once.
    #: (untyped) -> bool
    def __mt_flunk(msg) = __assert(false, -> { __s(msg || "Epic Fail!") })

    # port: the default message is a lambda argument, not a block: a lambda can't call its method's block.
    #: (untyped, String?, ^() -> String) -> ^() -> String
    def message(msg, ending, default)
      lambda do
        custom_message = msg.nil? || __s(msg).empty? ? "" : "#{__s(msg)}.\n"
        "#{custom_message}#{default.call}#{ending}"
      end
    end

    #: (?untyped) -> bool
    def pass(_msg = nil)
      __mt_trace("pass")
      __assert(true, -> { "" })
    end

    #: (untyped, ?untyped) -> bool
    def refute(test, msg = nil)
      __mt_trace("refute", test)
      return __assert(!test, message(nil, ".", -> { "Expected #{mu_pp(test)} to not be truthy" })) if msg.nil?

      __assert(!test, -> { __s(msg) })
    end

    # @dynamic
    #: (untyped, ?untyped) -> bool
    def refute_empty(obj, msg = nil)
      __mt_trace("refute_empty", obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to not be empty" })
      __mt_respond_to(obj, :empty?)
      __assert(!obj.empty?, m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def refute_equal(exp, act, msg = nil)
      __mt_trace("refute_equal", exp, act)
      m = message(msg, ".", -> { "Expected #{mu_pp(act)} to not be equal to #{mu_pp(exp)}" })
      __assert(!(exp == act), m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped, ?untyped) -> bool
    def refute_in_delta(exp, act, delta = 0.001, msg = nil)
      __mt_trace("refute_in_delta", exp, act, delta)
      __mt_in_delta(exp, act, delta, msg, true)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped, ?untyped) -> bool
    def refute_in_epsilon(exp, act, epsilon = 0.001, msg = nil)
      __mt_trace("refute_in_epsilon", exp, act, epsilon)
      __mt_in_delta(exp, act, (exp.abs < act.abs ? exp.abs : act.abs) * epsilon, msg, true)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def refute_includes(obj, sub, msg = nil)
      __mt_trace("refute_includes", obj, sub)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to not include #{mu_pp(sub)}" })
      __mt_respond_to(obj, :include?)
      __assert(!obj.include?(sub), m)
    end

    #: (untyped, untyped, ?untyped) -> bool
    def refute_instance_of(cls, obj, msg = nil)
      __mt_trace("refute_instance_of", cls, obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to not be an instance of #{__s(cls)}" })
      __assert(__mt_class_name(obj) != __s(cls), m)
    end

    #: (untyped, untyped, ?untyped) -> bool
    def refute_kind_of(cls, obj, msg = nil)
      __mt_trace("refute_kind_of", cls, obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to not be a kind of #{__s(cls)}" })
      __assert(!obj.kind_of?(cls), m)
    end

    #: (untyped, untyped, ?untyped) -> bool
    def refute_match(matcher, obj, msg = nil)
      __mt_trace("refute_match", matcher, obj)
      re = __mt_regexp(matcher)
      m = message(msg, ".", -> { "Expected #{mu_pp(re)} to not match #{mu_pp(obj)}" })
      __mt_respond_to(re, :=~)
      __assert(!__mt_match(re, obj), m)
    end

    #: (untyped, ?untyped) -> bool
    def refute_nil(obj, msg = nil)
      __mt_trace("refute_nil", obj)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to not be nil" })
      __assert(!obj.nil?, m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped, ?untyped) -> bool
    def refute_operator(o1, op, o2 = Assertions::UNDEFINED, msg = nil)
      if Assertions::UNDEFINED.equal?(o2)
        __mt_trace("refute_predicate", o1, op)
        return __mt_predicate(o1, op, msg, true)
      end

      __mt_trace("refute_operator", o1, op, o2)
      __mt_respond_to(o1, op)
      m = message(msg, ".", -> { "Expected #{mu_pp(o1)} to not be #{__s(op)} #{mu_pp(o2)}" })
      __assert(!__mt_send(o1, op, o2), m)
    end

    #: (String, ?untyped) -> bool
    def refute_path_exists(path, msg = nil)
      __mt_trace("refute_path_exists", path)
      m = message(msg, ".", -> { "Expected path '#{__s(path)}' to not exist" })
      __assert(!File.exist?(path), m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def refute_predicate(o1, op, msg = nil)
      __mt_trace("refute_predicate", o1, op)
      __mt_predicate(o1, op, msg, true)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def refute_respond_to(obj, meth, msg = nil)
      __mt_trace("refute_respond_to", obj, meth)
      m = message(msg, ".", -> { "Expected #{mu_pp(obj)} to not respond to #{__s(meth)}" })
      __assert(!obj.respond_to?(meth), m)
    end

    # @dynamic
    #: (untyped, untyped, ?untyped) -> bool
    def refute_same(exp, act, msg = nil)
      __mt_trace("refute_same", exp, act)
      m = message(msg, ".", lambda do
        "Expected %s (oid=%d) to not be the same as %s (oid=%d)" % [mu_pp(act), act.object_id, mu_pp(exp), exp.object_id]
      end)
      __assert(!exp.equal?(act), m)
    end

    #: (?untyped, ?untyped) -> bool
    def skip(msg = nil, _ignored = nil)
      raise Minitest::Skip, msg.nil? ? "Skipped, no message given" : __s(msg)
    end

    # port: `Regexp.new Regexp.escape matcher if String === matcher`.
    #: (untyped) -> untyped
    def __mt_regexp(matcher) = matcher.is_a?(String) ? /#{Regexp.escape(matcher)}/ : matcher

    # port: `matcher =~ obj`, then Regexp.last_match.
    # @dynamic
    #: (untyped, untyped) -> MatchData?
    def __mt_match(re, obj)
      return nil if obj.nil?

      re.match(__s(obj))
    end

    # port: `recv.__send__(op, *args)`. rbSendByName would keep every method of every class in the build, so user classes answer through their _Call tables and core values through the common operators and predicates here.
    # @dynamic
    #: (untyped, untyped, *untyped) -> untyped
    def __mt_send(recv, op, *args)
      name = op.to_s
      arg = args.first
      if args.size == 1
        case name
        when "==" then return recv == arg
        when "!=" then return recv != arg
        when "<" then return recv < arg
        when "<=" then return recv <= arg
        when ">" then return recv > arg
        when ">=" then return recv >= arg
        when "===" then return recv === arg
        when "=~" then return recv =~ arg
        when "eql?" then return recv.eql?(arg)
        when "equal?" then return recv.equal?(arg)
        when "include?" then return recv.include?(arg)
        when "member?" then return recv.member?(arg)
        when "key?" then return recv.key?(arg)
        when "start_with?" then return recv.start_with?(arg)
        when "end_with?" then return recv.end_with?(arg)
        when "is_a?", "kind_of?" then return recv.is_a?(arg)
        when "instance_of?" then return recv.instance_of?(arg)
        when "respond_to?" then return recv.respond_to?(arg)
        end
      end
      if args.empty?
        case name
        when "empty?" then return recv.empty?
        when "nil?" then return recv.nil?
        when "zero?" then return recv.zero?
        when "positive?" then return recv.positive?
        when "negative?" then return recv.negative?
        when "even?" then return recv.even?
        when "odd?" then return recv.odd?
        when "frozen?" then return recv.frozen?
        when "any?" then return recv.any?
        when "none?" then return recv.none?
        when "finite?" then return recv.finite?
        when "nan?" then return recv.nan?
        end
      end
      __mt_call(recv, name, args)
    end

    #: (untyped, String, Array[untyped]) -> untyped
    def __mt_call(recv, name, args) = %x{ return rbMtCall(recv, string(name), *args...) }

    # port: to_s, `===` and class names on untyped operands go through Go helpers (rbToS, rbIsInstanceOf, the class-name table): a dynamic call keeps a wrapper on every class in the program.
    #: (untyped) -> String
    def __s(obj) = %x{ return rbToS(obj) }

    #: (untyped, untyped) -> bool
    def __mt_kind_of(klass, obj) = %x{ return Boolean(rbIsInstanceOf(klass, obj)) }

    #: (untyped) -> String
    def __mt_class_name(obj) = %x{ return String(rbClassName(rbUnbox(obj))) }

    #: () -> bool
    def __mt_have_diff = %x{ return Boolean(rbMtHaveDiff()) }

    #: (String, String) -> String
    def __mt_diff(expect, butwas) = %x{ return rbMtDiff(string(expect), string(butwas)) }

    # Typed twins of the untyped assertions above (decision 93). A call
    # takes one when its arguments allow: `__<name>_same` when both sides
    # have one static type (T and T? count as T?), `__<name>_<class>` by the
    # collection's class, `__<name>_lit` for a literal operator symbol. Then
    # `==`, `include?`, `empty?` and the operator are compiled calls, and
    # no respond_to?-by-name table is kept. Each counts the assertions its
    # untyped twin would (assert_respond_to is one of them).

    # @rbs [T] (T) -> String
    def __mt_pp(obj) = %x{ return rbInspect(obj) }

    # @rbs [T] (T, T) -> bool
    def __mt_same(a, b) = %x{ return Boolean(rbIdentical(rbUnbox(any(a)), rbUnbox(any(b)))) }

    # @rbs [T] (T) -> Integer
    def __mt_oid(obj) = %x{ return rbObjectID(rbUnbox(any(obj))) }

    # @rbs [T] (T, T, ?untyped) -> bool
    def __assert_equal_same(exp, act, msg = nil)
      __mt_trace("assert_equal", exp, act)
      m = message(msg, nil, -> { diff(exp, act) })
      __assert(false, message(nil, ".", -> { "Use assert_nil if expecting nil" })) if exp.nil? # refute_nil with a proc message
      __assert(exp == act, m)
    end

    # @rbs [T] (T, T, ?untyped) -> bool
    def __refute_equal_same(exp, act, msg = nil)
      __mt_trace("refute_equal", exp, act)
      m = message(msg, ".", -> { "Expected #{__mt_pp(act)} to not be equal to #{__mt_pp(exp)}" })
      __assert(!(exp == act), m)
    end

    # @rbs [T] (T, T, ?untyped) -> bool
    def __assert_same_same(exp, act, msg = nil)
      __mt_trace("assert_same", exp, act)
      m = message(msg, ".", lambda do
        "Expected %s (oid=%d) to be the same as %s (oid=%d)" % [__mt_pp(act), __mt_oid(act), __mt_pp(exp), __mt_oid(exp)]
      end)
      __assert(false, message(nil, ".", -> { "Use assert_nil if expecting nil" })) if exp.nil? # refute_nil with a proc message
      __assert(__mt_same(exp, act), m)
    end

    # @rbs [T] (T, T, ?untyped) -> bool
    def __refute_same_same(exp, act, msg = nil)
      __mt_trace("refute_same", exp, act)
      m = message(msg, ".", lambda do
        "Expected %s (oid=%d) to not be the same as %s (oid=%d)" % [__mt_pp(act), __mt_oid(act), __mt_pp(exp), __mt_oid(exp)]
      end)
      __assert(!__mt_same(exp, act), m)
    end

    #: (Float, Float, ?Float, ?untyped) -> bool
    def __assert_in_delta_same(exp, act, delta = 0.001, msg = nil)
      __mt_trace("assert_in_delta", exp, act, delta)
      n = (exp - act).abs
      m = message(msg, ".", -> { "Expected |#{exp} - #{act}| (#{n}) to be <= #{delta}" })
      __assert(delta >= n, m)
    end

    #: (Float, Float, ?Float, ?untyped) -> bool
    def __refute_in_delta_same(exp, act, delta = 0.001, msg = nil)
      __mt_trace("refute_in_delta", exp, act, delta)
      n = (exp - act).abs
      m = message(msg, ".", -> { "Expected |#{exp} - #{act}| (#{n}) to not be <= #{delta}" })
      __assert(!(delta >= n), m)
    end

    # @rbs [E] (Array[E], E, ?untyped) -> bool
    def __assert_includes_array(collection, obj, msg = nil) = __mt_includes(collection, obj, collection.include?(obj), msg)

    # @rbs [K, V] (Hash[K, V], K, ?untyped) -> bool
    def __assert_includes_hash(collection, obj, msg = nil) = __mt_includes(collection, obj, collection.include?(obj), msg)

    # @rbs [E] (Set[E], E, ?untyped) -> bool
    def __assert_includes_set(collection, obj, msg = nil) = __mt_includes(collection, obj, collection.include?(obj), msg)

    #: (String, String, ?untyped) -> bool
    def __assert_includes_string(collection, obj, msg = nil) = __mt_includes(collection, obj, collection.include?(obj), msg)

    # @rbs [E] (Array[E], E, ?untyped) -> bool
    def __refute_includes_array(collection, obj, msg = nil) = __mt_excludes(collection, obj, collection.include?(obj), msg)

    # @rbs [K, V] (Hash[K, V], K, ?untyped) -> bool
    def __refute_includes_hash(collection, obj, msg = nil) = __mt_excludes(collection, obj, collection.include?(obj), msg)

    # @rbs [E] (Set[E], E, ?untyped) -> bool
    def __refute_includes_set(collection, obj, msg = nil) = __mt_excludes(collection, obj, collection.include?(obj), msg)

    #: (String, String, ?untyped) -> bool
    def __refute_includes_string(collection, obj, msg = nil) = __mt_excludes(collection, obj, collection.include?(obj), msg)

    # @rbs [C, T] (C, T, bool, untyped) -> bool
    def __mt_includes(collection, obj, test, msg)
      __mt_trace("assert_includes", collection, obj)
      m = message(msg, ".", -> { "Expected #{__mt_pp(collection)} to include #{__mt_pp(obj)}" })
      self.assertions += 1 # assert_respond_to(collection, :include?)
      __assert(test, m)
    end

    # @rbs [C, T] (C, T, bool, untyped) -> bool
    def __mt_excludes(collection, obj, test, msg)
      __mt_trace("refute_includes", collection, obj)
      m = message(msg, ".", -> { "Expected #{__mt_pp(collection)} to not include #{__mt_pp(obj)}" })
      self.assertions += 1 # assert_respond_to(collection, :include?)
      __assert(!test, m)
    end

    # @rbs [E] (Array[E], ?untyped) -> bool
    def __assert_empty_array(obj, msg = nil) = __mt_empty(obj, obj.empty?, msg)

    # @rbs [K, V] (Hash[K, V], ?untyped) -> bool
    def __assert_empty_hash(obj, msg = nil) = __mt_empty(obj, obj.empty?, msg)

    # @rbs [E] (Set[E], ?untyped) -> bool
    def __assert_empty_set(obj, msg = nil) = __mt_empty(obj, obj.empty?, msg)

    #: (String, ?untyped) -> bool
    def __assert_empty_string(obj, msg = nil) = __mt_empty(obj, obj.empty?, msg)

    # @rbs [E] (Array[E], ?untyped) -> bool
    def __refute_empty_array(obj, msg = nil) = __mt_not_empty(obj, obj.empty?, msg)

    # @rbs [K, V] (Hash[K, V], ?untyped) -> bool
    def __refute_empty_hash(obj, msg = nil) = __mt_not_empty(obj, obj.empty?, msg)

    # @rbs [E] (Set[E], ?untyped) -> bool
    def __refute_empty_set(obj, msg = nil) = __mt_not_empty(obj, obj.empty?, msg)

    #: (String, ?untyped) -> bool
    def __refute_empty_string(obj, msg = nil) = __mt_not_empty(obj, obj.empty?, msg)

    # @rbs [C] (C, bool, untyped) -> bool
    def __mt_empty(obj, test, msg)
      __mt_trace("assert_empty", obj)
      m = message(msg, ".", -> { "Expected #{__mt_pp(obj)} to be empty" })
      self.assertions += 1 # assert_respond_to(obj, :empty?)
      __assert(test, m)
    end

    # @rbs [C] (C, bool, untyped) -> bool
    def __mt_not_empty(obj, test, msg)
      __mt_trace("refute_empty", obj)
      m = message(msg, ".", -> { "Expected #{__mt_pp(obj)} to not be empty" })
      self.assertions += 1 # assert_respond_to(obj, :empty?)
      __assert(!test, m)
    end

    # The compiler's rewrite of `assert_operator a, :op, b` with a literal
    # operator: test is `a.op(b)`, already computed (decision 93).
    # @rbs [A, B] (A, String, B, untyped, untyped, bool) -> bool
    def __assert_operator_lit(o1, op, o2, test, msg, refute)
      __mt_trace(refute ? "refute_operator" : "assert_operator", o1, op.to_sym, o2)
      m = message(msg, ".", -> { "Expected #{__mt_pp(o1)} to #{refute ? "not " : ""}be #{op} #{__mt_pp(o2)}" })
      self.assertions += 1 # assert_respond_to(o1, op)
      __assert(refute ? !test : test, m)
    end

    # @rbs [A] (A, String, untyped, untyped, bool) -> bool
    def __assert_predicate_lit(o1, op, test, msg, refute)
      __mt_trace(refute ? "refute_predicate" : "assert_predicate", o1, op.to_sym)
      m = message(msg, ".", -> { "Expected #{__mt_pp(o1)} to #{refute ? "not " : ""}be #{op}" })
      self.assertions += 1 # assert_respond_to(o1, op)
      __assert(refute ? !test : test, m)
    end

    # @rbs [A] (A, String, bool, untyped, bool) -> bool
    def __assert_respond_to_lit(obj, meth, test, msg, refute)
      __mt_trace(refute ? "refute_respond_to" : "assert_respond_to", obj, meth.to_sym)
      m = if refute
        message(msg, ".", -> { "Expected #{__mt_pp(obj)} to not respond to #{meth}" })
      else
        message(msg, ".", -> { "Expected #{__mt_pp(obj)} (#{__mt_class_name(obj)}) to respond to ##{meth}" })
      end
      __assert(refute ? !test : test, m)
    end


    #: () -> Array[String]
    def self.runnable_methods
      methods = methods_matching(/^test_/)

      case run_order
      when :random, :parallel
        srand(Minitest.seed)
        methods.sort.shuffle
      when :alpha, :sorted
        methods.sort
      else
        raise "Unknown_order: %p" % [run_order]
      end
    end

    #: () -> Minitest::Result
    def run
      time_it do
        capture_exceptions do
          before_setup
          setup
          after_setup
          __run_test
        end

        capture_exceptions { before_teardown }
        capture_exceptions { teardown }
        capture_exceptions { after_teardown }
      end

      Result.from(self) # per contract
    end

    # port: `self.send self.name`.
    #: () -> void
    def __run_test = %x{ rbMtCall(self, string(self.Name())) }

    #: () -> void
    def before_setup = nil

    #: () -> void
    def setup = nil

    #: () -> void
    def after_setup = nil

    #: () -> void
    def before_teardown = nil

    #: () -> void
    def teardown = nil

    #: () -> void
    def after_teardown = nil

    #: () { () -> void } -> void
    def capture_exceptions
      yield
    rescue NoMemoryError, SignalException, SystemExit => e
      raise e
    rescue Exception => e
      failures << (e.is_a?(Assertion) ? e : UnexpectedError.new(e))
    end
  end

  # port: Expectation is a typed class, and each must_/wont_ method is written out: MRI generates them with infect_an_assertion's class_eval. target is untyped, as the assertions' operands are (issue #29).
  class Expectation < Object
    attr_reader :target #: untyped
    attr_reader :ctx #: Minitest::Test

    #: (untyped, Minitest::Test, (^() -> void)?) -> void
    def initialize(target, ctx, block)
      @target = target
      @ctx = ctx
      @block = block
    end

    # port: must_raise runs the block _ was given, where MRI finds it as a Proc target.
    #: () -> void
    def __call_block = %x{
      b := self._Minitest_Expectation().block
      if b == nil {
        panic(NewArgumentError(Ref(String("must_raise needs a block: _ { ... }.must_raise(...)"))))
      }
      (**b)() // a Proc? is a pointer to the Proc, itself a pointer to the func
    }

    #: (?untyped) -> bool
    def must_be_empty(msg = nil) = ctx.assert_empty(target, msg)

    #: (untyped, ?untyped) -> bool
    def must_equal(exp, msg = nil) = ctx.assert_equal(exp, target, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def must_be_close_to(exp, delta = 0.001, msg = nil) = ctx.assert_in_delta(exp, target, delta, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def must_be_within_delta(exp, delta = 0.001, msg = nil) = ctx.assert_in_delta(exp, target, delta, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def must_be_within_epsilon(exp, epsilon = 0.001, msg = nil) = ctx.assert_in_epsilon(exp, target, epsilon, msg)

    #: (untyped, ?untyped) -> bool
    def must_include(obj, msg = nil) = ctx.assert_includes(target, obj, msg)

    #: (untyped, ?untyped) -> bool
    def must_be_instance_of(cls, msg = nil) = ctx.assert_instance_of(cls, target, msg)

    #: (untyped, ?untyped) -> bool
    def must_be_kind_of(cls, msg = nil) = ctx.assert_kind_of(cls, target, msg)

    #: (untyped, ?untyped) -> MatchData?
    def must_match(matcher, msg = nil) = ctx.assert_match(matcher, target, msg)

    #: (?untyped) -> bool
    def must_be_nil(msg = nil) = ctx.assert_nil(target, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def must_be(op, o2 = Assertions::UNDEFINED, msg = nil) = ctx.assert_operator(target, op, o2, msg)

    #: (*untyped) -> Exception
    def must_raise(*exp) = ctx.assert_raises(*exp) { __call_block }

    #: (untyped, ?untyped) -> bool
    def must_respond_to(meth, msg = nil) = ctx.assert_respond_to(target, meth, msg)

    #: (untyped, ?untyped) -> bool
    def must_be_same_as(exp, msg = nil) = ctx.assert_same(exp, target, msg)

    #: (?untyped) -> bool
    def path_must_exist(msg = nil) = ctx.assert_path_exists(ctx.__s(target), msg)

    #: (?untyped) -> bool
    def path_wont_exist(msg = nil) = ctx.refute_path_exists(ctx.__s(target), msg)

    #: (?untyped) -> bool
    def wont_be_empty(msg = nil) = ctx.refute_empty(target, msg)

    #: (untyped, ?untyped) -> bool
    def wont_equal(exp, msg = nil) = ctx.refute_equal(exp, target, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def wont_be_close_to(exp, delta = 0.001, msg = nil) = ctx.refute_in_delta(exp, target, delta, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def wont_be_within_delta(exp, delta = 0.001, msg = nil) = ctx.refute_in_delta(exp, target, delta, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def wont_be_within_epsilon(exp, epsilon = 0.001, msg = nil) = ctx.refute_in_epsilon(exp, target, epsilon, msg)

    #: (untyped, ?untyped) -> bool
    def wont_include(obj, msg = nil) = ctx.refute_includes(target, obj, msg)

    #: (untyped, ?untyped) -> bool
    def wont_be_instance_of(cls, msg = nil) = ctx.refute_instance_of(cls, target, msg)

    #: (untyped, ?untyped) -> bool
    def wont_be_kind_of(cls, msg = nil) = ctx.refute_kind_of(cls, target, msg)

    #: (untyped, ?untyped) -> bool
    def wont_match(matcher, msg = nil) = ctx.refute_match(matcher, target, msg)

    #: (?untyped) -> bool
    def wont_be_nil(msg = nil) = ctx.refute_nil(target, msg)

    #: (untyped, ?untyped, ?untyped) -> bool
    def wont_be(op, o2 = Assertions::UNDEFINED, msg = nil) = ctx.refute_operator(target, op, o2, msg)

    #: (untyped, ?untyped) -> bool
    def wont_respond_to(meth, msg = nil) = ctx.refute_respond_to(target, meth, msg)

    #: (untyped, ?untyped) -> bool
    def wont_be_same_as(exp, msg = nil) = ctx.refute_same(exp, target, msg)
  end

  # port: the DSL (describe, it, let, before, after, subject) is compiled into classes and methods (decision 83); what is left is the expectation entry point.
  class Spec < Test
    # MRI's Spec::DSL::InstanceMethods#_, with value and expect as its aliases.
    #: (?untyped) ?{ () -> void } -> Minitest::Expectation
    def _(value = nil) = %x{ return NewMinitest_Expectation(value, self, rbProcOrNil(blk)) }

    #: (?untyped) ?{ () -> void } -> Minitest::Expectation
    def value(value = nil) = %x{ return NewMinitest_Expectation(value, self, rbProcOrNil(blk)) }

    #: (?untyped) ?{ () -> void } -> Minitest::Expectation
    def expect(value = nil) = %x{ return NewMinitest_Expectation(value, self, rbProcOrNil(blk)) }
  end
end
