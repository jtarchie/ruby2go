# rbs_inline: enabled

# Psych 5 over goccy/go-yaml (decision 376): loads by Psych's YAML 1.1 rules; dump text is rugaroo's own emitter, not libyaml's.

require_relative "date"

module Kernel
  #: (?untyped) -> String
  def to_yaml(_opts = nil) = Psych.dump(self)
end

module Psych
  VERSION = "5.4.0"

  class Exception < RuntimeError; end
  class BadAlias < Exception; end

  class AliasesNotEnabled < BadAlias
    #: () -> void
    def initialize = super("Alias parsing was not enabled. To enable it, pass `aliases: true` to `Psych::load` or `Psych::safe_load`.")
  end

  class AnchorNotDefined < BadAlias
    #: (String) -> void
    def initialize(anchor_name) = super("An alias referenced an unknown anchor: #{anchor_name}")
  end

  class DisallowedClass < Exception
    #: (String, String) -> void
    def initialize(action, klass_name) = super("Tried to #{action} unspecified class: #{klass_name}")
  end

  # problem/context are go-yaml's wording, not libyaml's (decision 376): match on class, line and column.
  class SyntaxError < Exception
    attr_reader :file #: String?
    attr_reader :line #: Integer
    attr_reader :column #: Integer
    attr_reader :offset #: Integer
    attr_reader :problem #: String
    attr_reader :context #: String

    #: (String?, Integer, Integer, Integer, String, String) -> void
    def initialize(file, line, col, offset, problem, context)
      err = [problem, context].compact.join(" ")
      super("(#{file || "<unknown>"}): #{err} at line #{line} column #{col}")
      @file = file
      @line = line
      @column = col
      @offset = offset
      @problem = problem
      @context = context
    end
  end

  # opts: Psych's keywords (permitted_classes, permitted_symbols, aliases, filename, fallback, symbolize_names, freeze, strict_integer, parse_symbols).
  #: (untyped, ?Hash[Symbol, untyped]) -> untyped
  def self.safe_load(yaml, opts = {}) = __load(yaml, opts, [])

  #: (untyped, ?Hash[Symbol, untyped]) -> untyped
  def self.load(yaml, opts = {}) = __load(yaml, opts, ["Symbol"])

  # Psych's unsafe_load revives any class; here every core value and alias loads, and a `!ruby/object` tag raises NotImplementedError (no runtime reflection).
  #: (untyped, ?Hash[Symbol, untyped]) -> untyped
  def self.unsafe_load(yaml, opts = {}) = __load(yaml, opts, nil)

  #: (String, ?Hash[Symbol, untyped]) -> untyped
  def self.load_file(filename, opts = {}) = load(File.read(filename), {filename: filename}.merge(opts))

  #: (String, ?Hash[Symbol, untyped]) -> untyped
  def self.safe_load_file(filename, opts = {}) = safe_load(File.read(filename), {filename: filename}.merge(opts))

  #: (String, ?Hash[Symbol, untyped]) -> untyped
  def self.unsafe_load_file(filename, opts = {}) = unsafe_load(File.read(filename), {filename: filename}.merge(opts))

  # Every document, unrestricted as Psych's load_stream (ponytail: no block form; add a yield over the Array when a program needs it).
  #: (untyped, ?Hash[Symbol, untyped]) -> Array[untyped]
  def self.load_stream(yaml, opts = {}) = __load_stream(__source(yaml), opts)

  #: (String, Hash[Symbol, untyped]) -> Array[untyped]
  def self.__load_stream(src, opts) = %x{ return rbYAMLLoadStream(string(src), opts) }

  # Psych reads an IO-like source.
  #: (untyped) -> String
  # @dynamic
  def self.__source(yaml)
    src = yaml.respond_to?(:read) ? yaml.read : yaml
    raise TypeError, "no implicit conversion of #{src.class} into String" unless src.is_a?(String)

    src
  end

  # permitted: the class names a restricted load accepts unless permitted_classes overrides them; nil for unsafe_load.
  #: (untyped, Hash[Symbol, untyped], Array[String]?) -> untyped
  def self.__load(yaml, opts, permitted)
    classes = opts[:permitted_classes] #: Array[untyped]?
    names = classes.nil? ? permitted || [] : classes.map { |c| c.to_s } #: Array[String]
    symbols = opts[:permitted_symbols] #: Array[untyped]?
    syms = symbols.nil? ? [] : symbols.map { |s| s.to_s } #: Array[String]
    __load_go(__source(yaml), opts, permitted.nil?, names, syms)
  end

  #: (String, Hash[Symbol, untyped], bool, Array[String], Array[String]) -> untyped
  def self.__load_go(src, opts, unsafe, permitted, symbols) = %x{ return rbYAMLLoad(string(src), opts, bool(unsafe), permitted, symbols) }

  # dump(o, io) writes to io and returns it; options are accepted and ignored (rbYAMLDump picks the layout).
  #: (untyped, ?untyped, ?untyped) -> untyped
  # @dynamic
  def self.dump(o, io = nil, _opts = nil)
    yaml = __dump(o)
    return yaml if io.nil? || io.is_a?(Hash)

    io.write(yaml)
    io
  end

  #: (untyped, ?untyped, ?untyped) -> untyped
  def self.safe_dump(o, io = nil, opts = nil) = dump(o, io, opts)

  #: (*untyped) -> String
  def self.dump_stream(*objects) = objects.map { |o| __dump(o) }.join

  #: (untyped) -> String
  def self.__dump(o) = %x{ return rbYAMLDump(o) }
end

YAML = Psych
