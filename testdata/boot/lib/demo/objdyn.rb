# frozen_string_literal: true

require "stringio"
require "pathname"

# Run-time Ruby that bindata, elftools and patchelf lean on, compiled ahead of time (decision 324).
module DemoObjDyn
  # Per-object accessors whose names are data: define_singleton_method(computed name) on self.
  class Fields
    def initialize(names)
      @vals = {}
      names.each_with_index { |n, i| define_accessors_for(n, i) }
    end

    def define_accessors_for(name, index)
      define_singleton_method(name) { @vals[index] }
      define_singleton_method("#{name}=") { |*v| @vals[index] = v.first }
    end
  end

  # A singleton override of a real method on another object, by literal name.
  class Parser
    def initialize = @seen = []
    attr_reader :seen

    def add(*args) = (@seen << args.join("-"))
  end

  def self.redirect(parser, into)
    parser.define_singleton_method(:add) do |*args|
      into << "redirected:#{args.join("-")}"
    end
  end

  # extend M on an object at run time, M's def calling super.
  module Loud
    def word = "#{super}!"
    def extra = "extra"
  end

  class Speaker
    def initialize(loud)
      extend Loud if loud
    end

    def word = "hi"
  end

  # A DSL block in a class body, instance_eval'd by the gem on a fresh object.
  class Collector
    def initialize = @items = []
    attr_reader :items

    def item(x) = @items << x
  end

  module DSL
    def section(name, &block)
      c = Collector.new
      c.instance_eval(&block)
      (@sections ||= {})[name] = c.items
    end

    def sections = @sections
  end

  class Doc
    extend DSL
    section(:a) do
      item 1
      item 2
    end
  end

  # Ivars named from a parameter; a nil-until-set typed ivar under ||=.
  class TopLevel
    def set(sym, v) = instance_variable_set("@tl_#{sym}", v)
    def get(sym) = instance_variable_get("@tl_#{sym}")

    def go
      set(:in_read, true)
      set(:depth, 3)
      [get(:in_read), get(:depth)]
    end
  end

  class Memo
    def initialize(build) = (@build = build)

    def obj
      @obj ||= @build.new
    end
  end

  # 64-bit masks: a gem's Integer#<< wraps, literal arithmetic folds; the low bits agree with MRI's Bignums.
  def self.bits
    mask = (1 << 64) - 1
    n = 64
    [mask & 0xffff, ((1 << n) - 1) & 0xff, (1 << (n - 1)) & 1, ("\xff" * 8).b.unpack1("Q<") & 0xfff]
  end

  # String edits on a local, and bytes of a binary String.
  def self.strings
    s = +"abcdefghij"
    s[2..4] = "XY"
    t = +"0123"
    t[1, 2] = "r"
    bin = "\x01\xe6\x01\x00\xff".b
    [s, t, bin.size, bin[1, 2].bytes, bin[4].bytes]
  end

  # Integer arithmetic with an object answering coerce and to_int through method_missing.
  class Num
    def initialize(v) = @v = v
    def method_missing(sym, *args) = @v.respond_to?(sym) ? @v.__send__(sym, *args) : super
    def respond_to_missing?(sym, priv = false) = @v.respond_to?(sym, priv) || super
  end

  def self.numbers(n)
    io = StringIO.new("abcdef")
    io.pos = n
    steps = []
    0.step do |i|
      steps << i
      break if i == 2
    end
    [3 * n, io.read(2), Array.new(2), steps]
  end

  # refine is a reopening; using a no-op.
  module Refined
    refine String do
      def shout = upcase + "!"
    end
  end
  using Refined

  def self.refined = "hey".shout

  # A block's `return false` unwinds to the method with the method's Boolean, not Go's bool (Utils::Output.claim_warning).
  SEEN = {} # rubocop:disable Style/MutableConstant
  LOCK = Mutex.new

  def self.claim(message)
    LOCK.synchronize do
      return false if SEEN[message]

      SEEN[message] = true
    end
  end

  # A module's `"#{self}"` whose to_s is dispatched at run time is a String (Homebrew's ObserverPathnameExtension).
  module Shown
    def shown = "mkdir -p #{self}"
  end

  class ShownPath < Pathname
    include Shown
  end

  def self.run
    f = Fields.new(%i[a b])
    f.b = 7
    out = [f.a, f.b, f.respond_to?(:b)]
    seen = []
    pr = Parser.new
    pr.add(1, 2)
    redirect(pr, seen)
    pr.add(3, 4)
    out << pr.seen << seen
    out << Speaker.new(false).word << Speaker.new(true).word << Speaker.new(true).extra
    out << Doc.sections
    out << TopLevel.new.go
    m = Memo.new(Collector)
    out << m.obj.equal?(m.obj)
    out << bits << strings << numbers(Num.new(2)) << refined << [claim("w"), claim("w")] << ShownPath.new("/tmp/x").shown
    out
  end
end
