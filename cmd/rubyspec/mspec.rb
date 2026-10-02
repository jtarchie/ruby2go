# rbs_inline: enabled
# mspec's expectations over minitest, for running ruby/spec under rb2go (cmd/rubyspec).
# The runner rewrites what must be static first: context → describe, guards, shared specs.
require "minitest/autorun"
require "fileutils"

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
  def frozen? = check(@actual.frozen?, "to be frozen")

  #: () -> bool
  def empty? = check(@actual.empty?, "to be empty")

  #: () -> bool
  def nil? = check(@actual.nil?, "to be nil")

  #: () -> bool
  def nan? = check(@actual.nan?, "to be NaN")

  #: () -> bool
  def zero? = check(@actual.zero?, "to be zero")

  #: () -> bool
  def finite? = check(@actual.finite?, "to be finite")

  #: () -> bool
  def infinite? = check(!@actual.infinite?.nil?, "to be infinite")
end

# `-> { ... }.should.raise(E)`, which the runner rewrites to ProcExpectation.new(-> { ... }, true).raise(E): a Proc has no methods to add.
class ProcExpectation
  #: (^() -> untyped, bool) -> void
  def initialize(actual, positive)
    @actual = actual
    @positive = positive
  end

  #: (?untyped, ?String?) -> bool
  def raise(klass = StandardError, message = nil)
    got = nil #: Exception?
    begin
      @actual.call
    rescue Exception => e # rubocop:disable Lint/RescueException
      got = e
    end
    ok = !got.nil? && got.is_a?(klass) && (message.nil? || got.message == message)
    return true if ok == @positive

    mspec_fail "Expected #{@positive ? "" : "no "}#{klass.inspect}#{message.nil? ? "" : " #{message.inspect}"}, got #{got.inspect}"
  end
end

# The expectations' own #raise hides Kernel's inside them.
#: (String) -> bot
def mspec_fail(message) = raise(Minitest::Assertion, message)

module Kernel
  #: () -> SpecExpectation
  def should = SpecExpectation.new(self, true)

  #: () -> SpecExpectation
  def should_not = SpecExpectation.new(self, false)
end

# No bignum_value: there is no Bignum (decision 35), so a spec using it is a compile error, which the runner reports as unsupported.

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

SPEC_TEMP_DIR = ENV["SPEC_TEMP_DIR"] || File.join(Dir.pwd, "rubyspec_temp", Process.pid.to_s) #: String
SPEC_TEMP_UNIQUIFIER = [0] #: Array[Integer]

# A path under SPEC_TEMP_DIR, made unique by a counter before its basename.
#: (String, ?bool) -> String
def tmp(name, uniquify = true)
  FileUtils.mkdir_p(SPEC_TEMP_DIR)
  if uniquify && !name.empty?
    SPEC_TEMP_UNIQUIFIER[0] += 1
    slash = name.rindex("/")
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
def touch(name, mode = "w", &blk)
  FileUtils.mkdir_p(File.dirname(name))
  File.open(name, mode) { |f| blk&.call(f) }
end

#: (String, String) -> void
def cp(source, dest) = FileUtils.cp(source, dest)

# The fixtures directory beside file (a shared spec's is its parent's), joined with args.
#: (String, *String) -> String
def fixture(file, *args)
  path = File.dirname(file).delete_suffix("/shared")
  fixtures = path.end_with?("/fixtures") ? "" : "fixtures"
  File.join(File.expand_path(path), fixtures, *args)
end

#: (String, ?String) -> File
def new_io(name, mode = "w:utf-8") = File.new(name, mode)

#: (?String) -> bot
def flunk(message = "This example is a failure") = mspec_fail(message)

at_exit { FileUtils.rm_rf(SPEC_TEMP_DIR) }
