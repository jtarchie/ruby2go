# rbs_inline: enabled

# mspec's expectations over minitest, for running ruby/spec under rugaroo (cmd/rubyspec).
# The runner rewrites what must be static first: context → describe, guards, shared specs.
require 'minitest/autorun'
require 'fileutils'
require 'stringio'

# `actual.should == x`, `actual.should.equal?(x)`, `-> { }.should.raise(E)`; should_not negates.
class SpecExpectation
  #: (untyped, bool) -> void
  def initialize(actual, positive)
    @actual = actual
    @positive = positive
  end

  #: (bool, String) -> bool
  def check(ok, what)
    return true if ok == @positive

    mspec_fail "Expected #{@actual.inspect} #{@positive ? "" : "not "}#{what}"
  end

  #: (untyped) -> bool
  def ==(other) = check(@actual == other, "== #{other.inspect}")

  #: (untyped) -> bool
  def !=(other) = check(@actual != other, "!= #{other.inspect}")

  #: (untyped) -> bool
  def =~(other) = check(!(@actual =~ other).nil?, "=~ #{other.inspect}")

  # `x.should be_close(y, TOLERANCE)`, which the runner rewrites to `x.should.be_close(y, TOLERANCE)`
  #: (untyped, untyped) -> bool
  def be_close(expected, tolerance)
    check((@actual - expected).abs <= tolerance, "to be within +/- #{tolerance} of #{expected.inspect}")
  end

  #: (untyped) -> bool
  def <(other) = check(@actual < other, "< #{other.inspect}")

  #: (untyped) -> bool
  def <=(other) = check(@actual <= other, "<= #{other.inspect}")

  #: (untyped) -> bool
  def >(other) = check(@actual > other, "> #{other.inspect}")

  #: (untyped) -> bool
  def >=(other) = check(@actual >= other, ">= #{other.inspect}")

  #: (untyped) -> bool
  def equal?(other) = check(@actual.equal?(other), "to be equal? #{other.inspect}")

  #: (untyped) -> bool
  def eql?(other) = check(@actual.eql?(other), "to be eql? #{other.inspect}")

  #: (untyped) -> bool
  def include?(other) = check(@actual.include?(other), "to include #{other.inspect}")

  #: (untyped) -> bool
  def is_a?(other) = check(@actual.is_a?(other), "to be a #{other.inspect}")

  #: (untyped) -> bool
  def kind_of?(other) = check(@actual.is_a?(other), "to be a #{other.inspect}")

  #: (untyped) -> bool
  def instance_of?(other) = check(@actual.instance_of?(other), "to be an instance of #{other.inspect}")

  #: (untyped) -> bool
  def respond_to?(other) = check(@actual.respond_to?(other), "to respond to #{other.inspect}")

  #: () -> bool
  def frozen? = check(@actual.frozen?, 'to be frozen')

  # mspec's predicate matchers with an argument (`File.should.exist?(path)`), each the actual's own answer.
  #: () -> bool
  def eof? = check(@actual.eof?, 'to be at eof')

  #: () -> bool
  def binmode? = check(@actual.binmode?, 'to be in binmode')

  #: (untyped) -> bool
  def exist?(x) = check(@actual.exist?(x), "exist? #{x.inspect}")

  #: (untyped) -> bool
  def start_with?(x) = check(@actual.start_with?(x), "to start with #{x.inspect}")

  #: (untyped) -> bool
  def end_with?(x) = check(@actual.end_with?(x), "to end with #{x.inspect}")

  #: (untyped) -> bool
  def const_defined?(x) = check(@actual.const_defined?(x), "to define #{x.inspect}")

  #: (untyped) -> bool
  def directory?(x) = check(@actual.directory?(x), "directory? #{x.inspect}")

  #: (untyped) -> bool
  def file?(x) = check(@actual.file?(x), "file? #{x.inspect}")

  #: (untyped) -> bool
  def method_defined?(x) = check(@actual.method_defined?(x), "to define method #{x.inspect}")

  #: (untyped) -> bool
  def instance_variable_defined?(x) = check(@actual.instance_variable_defined?(x), "to define #{x.inspect}")

  #: () -> bool
  def empty? = check(@actual.empty?, 'to be empty')

  #: () -> bool
  def nil? = check(@actual.nil?, 'to be nil')

  #: () -> bool
  def nan? = check(@actual.nan?, 'to be NaN')

  #: () -> bool
  def zero? = check(@actual.zero?, 'to be zero')

  #: () -> bool
  def finite? = check(@actual.finite?, 'to be finite')

  #: () -> bool
  def infinite? = check(!@actual.infinite?.nil?, 'to be infinite')
end

# `-> { ... }.should.raise(E)`, which the runner rewrites to ProcExpectation.new(-> { ... }, true).raise(E): a Proc has no methods to add.
class ProcExpectation
  #: (^() -> untyped, bool) -> void
  def initialize(actual, positive)
    @actual = actual
    @positive = positive
  end

  # A block sees the exception raised, as mspec's raise_error { |e| ... }.
  #: (?untyped, ?untyped) ?{ (Exception) -> void } -> bool
  def raise(klass = StandardError, message = nil, &blk)
    got = nil #: Exception?
    begin
      @actual.call
    rescue Exception => e # rubocop:disable Lint/RescueException
      got = e
    end
    ok = !got.nil? && got.is_a?(klass) && (message.nil? || mspec_match(got.message, message)) # a String equals, a Regexp matches
    blk.call(got) if ok && @positive && blk && got
    return true if ok == @positive

    mspec_fail "Expected #{@positive ? "" : "no "}#{klass.inspect}#{message.nil? ? "" : " #{message.inspect}"}, got #{got.inspect}"
  end

  # mspec's complain: the block's warnings ($stderr) match a String exactly, a Regexp, or for nil are any at all
  #: (?untyped, ?verbose: bool) -> bool
  def complain(expected = nil, verbose: false)
    old_err = $stderr
    old_verbose = $VERBOSE
    buf = StringIO.new
    $stderr = buf
    $VERBOSE = verbose
    begin
      @actual.call
    ensure
      $stderr = old_err
      $VERBOSE = old_verbose
    end
    got = buf.string
    ok = expected.nil? ? !got.empty? : mspec_match(got, expected)
    return true if ok == @positive

    mspec_fail "Expected warning #{@positive ? "" : "not "}#{expected.nil? ? "" : expected.inspect}, got #{got.inspect}"
  end

  # mspec's output: what the block prints to $stdout and $stderr; nil checks nothing
  #: (?untyped, ?untyped) -> bool
  def output(stdout = nil, stderr = nil)
    old_out = $stdout
    old_err = $stderr
    out = StringIO.new
    err = StringIO.new
    $stdout = out
    $stderr = err
    begin
      @actual.call
    ensure
      $stdout = old_out
      $stderr = old_err
    end
    ok = (stdout.nil? || mspec_match(out.string, stdout)) && (stderr.nil? || mspec_match(err.string, stderr))
    return true if ok == @positive

    mspec_fail "Expected output #{stdout.inspect}, #{stderr.inspect}, got #{out.string.inspect}, #{err.string.inspect}"
  end
end

# A String must equal, a Regexp must match.
#: (String, untyped) -> bool
def mspec_match(got, expected) = expected.is_a?(Regexp) ? got.match?(expected) : got == expected

# mspec's failure exceptions, which specs raise themselves (`raise ExpectationNotMetError, "..."` in a path that must not run).
class SpecExpectationNotMetError < StandardError
end

class ExpectationNotMetError < SpecExpectationNotMetError
end

# The expectations' own #raise hides Kernel's inside them.
#: (String) -> bot
def mspec_fail(message) = raise(Minitest::Assertion, message)

# nil is an Object in Ruby: `x.should == nil` on an optional x must reach these when x is nil.
class NilClass
  #: () -> SpecExpectation
  def should = SpecExpectation.new(nil, true)

  #: () -> SpecExpectation
  def should_not = SpecExpectation.new(nil, false)
end

module Kernel
  #: () -> SpecExpectation
  def should = SpecExpectation.new(self, true)

  #: () -> SpecExpectation
  def should_not = SpecExpectation.new(self, false)
end

# No bignum_value: there is no Bignum (decision 35), so a spec using it is a compile error, which the runner reports as unsupported; under RUGAROO_INT_OVERFLOW=promote the runner appends one (decision 372).

TOLERANCE = 0.00003 #: Float

#: () -> Integer
def fixnum_max = 0x3fff_ffff_ffff_ffff

#: () -> Integer
def fixnum_min = -0x4000_0000_0000_0000

#: () -> Float
def nan_value = 0.0 / 0.0

#: () -> Float
def infinity_value = 1.0 / 0.0

# mspec's helpers (mspec/lib/mspec/helpers), typed.

# What a spec records to check later, as mspec's.
module ScratchPad
  @@record = nil #: untyped

  #: () -> nil
  def self.clear
    @@record = nil
  end

  #: (untyped) -> untyped
  def self.record(arg)
    @@record = arg
  end

  #: (untyped) -> untyped
  def self.<<(arg)
    @@record << arg
  end

  #: () -> untyped
  def self.recorded = @@record
end

SPEC_TEMP_DIR = ENV['SPEC_TEMP_DIR'] || File.join(Dir.pwd, 'rubyspec_temp', Process.pid.to_s) #: String
SPEC_TEMP_UNIQUIFIER = [0] #: Array[Integer]

# A path under SPEC_TEMP_DIR, made unique by a counter before its basename.
#: (String, ?bool) -> String
def tmp(name, uniquify = true)
  FileUtils.mkdir_p(SPEC_TEMP_DIR)
  if uniquify && !name.empty?
    SPEC_TEMP_UNIQUIFIER[0] += 1
    slash = name.rindex('/')
    at = slash.nil? ? 0 : slash + 1
    name = "#{name[0, at]}#{SPEC_TEMP_UNIQUIFIER[0]}-#{name[at..]}"
  end
  File.join(SPEC_TEMP_DIR, name)
end

#: (String) -> void
def mkdir_p(path) = FileUtils.mkdir_p(path)

# Removes paths under SPEC_TEMP_DIR only, as mspec's guard.
#: (*String) -> void
def rm_r(*paths)
  paths.each do |path|
    path = File.expand_path(path)
    raise ArgumentError, "#{path} is not prefixed by #{SPEC_TEMP_DIR}" unless path.start_with?(SPEC_TEMP_DIR)

    FileUtils.rm_rf(path)
  end
end

#: (String, ?String) ?{ (File) -> void } -> void
def touch(name, mode = 'w', &blk)
  FileUtils.mkdir_p(File.dirname(name))
  File.open(name, mode) { |f| blk&.call(f) }
end

#: (String, String) -> void
def cp(source, dest) = FileUtils.cp(source, dest)

# The fixtures directory beside file (a shared spec's is its parent's), joined with args.
#: (String, *String) -> String
def fixture(file, *args)
  path = File.dirname(file).delete_suffix('/shared')
  fixtures = path.end_with?('/fixtures') ? '' : 'fixtures'
  File.join(File.expand_path(path), fixtures, *args)
end

# A mode String, or mspec's Hash of open options (only its mode: is read).
#: (String, ?(String | Hash[Symbol, untyped])) -> File
def new_io(name, mode = 'w:utf-8')
  return File.new(name, mode) if mode.is_a?(String)

  File.new(name, (mode[:mode] || 'r').to_s)
end

#: (String, ?String) -> Integer
def new_fd(name, mode = 'w:utf-8') = IO.sysopen(name, mode)

# Runs the block with ENV["TZ"] set to name (offset hours east, which POSIX TZ writes west-positive), as mspec's.
#: [T] (String, ?Integer?, ?String) { () -> T } -> T
def with_timezone(name, offset = nil, daylight_saving_zone = '')
  zone = name.dup
  zone += "#{-offset}:00:00" if offset
  zone += daylight_saving_zone
  old = ENV['TZ']
  ENV['TZ'] = zone
  begin
    yield
  ensure
    ENV['TZ'] = old
  end
end

# ruby_exe and ruby_cmd hand Ruby source to a fresh MRI process; a compiled program has no Ruby to run it (no eval), so each such example skips for good.
#: (*untyped, **untyped) -> bot
def ruby_exe(*, **) = raise(Minitest::Skip, 'rugaroo: ruby_exe runs Ruby source in a subprocess (no eval)')

#: (*untyped, **untyped) -> bot
def ruby_cmd(*, **) = raise(Minitest::Skip, 'rugaroo: ruby_cmd runs Ruby source in a subprocess (no eval)')

# mspec's suppress_warning: the block runs with $VERBOSE nil, so warn prints nothing.
#: [T] () { () -> T } -> T
def suppress_warning
  old = $VERBOSE
  $VERBOSE = nil
  begin
    yield
  ensure
    $VERBOSE = old
  end
end

# platform_is/platform_is_not as values (the runner decides their block forms statically): this machine's OS.
#: (*Symbol) -> bool
def platform_is(*names)
  os = RUBY_PLATFORM.split('-').last.to_s.sub(/\d+\z/, '')
  names.any? { |n| n.to_s == os || (n == :bsd && os.end_with?('bsd')) || %i[posix unix].include?(n) && os != 'windows' }
end

#: (*Symbol) -> bool
def platform_is_not(*names) = !platform_is(*names)

# mspec's mock_to_path for a computed path (the runner makes a class of a literal one): an object whose to_path answers it.
class MSpecMockToPath
  #: (String) -> void
  def initialize(path)
    @path = path
  end

  #: () -> String
  def to_path = @path
end

#: (String) -> MSpecMockToPath
def mock_to_path(path) = MSpecMockToPath.new(path)

# A mock the runner did not make a class of (it has no expectations, or ones it cannot read): a plain object of its own.
#: (?untyped) -> Object
def mock(_name = nil) = Object.new

#: (?String) -> bot
def flunk(message = 'This example is a failure') = mspec_fail(message)

at_exit { FileUtils.rm_rf(SPEC_TEMP_DIR) }
