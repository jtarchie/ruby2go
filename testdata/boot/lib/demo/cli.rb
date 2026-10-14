require "sorbet-runtime"

# Homebrew's command DSL: a `T.proc.bind(Parser)` block stored in an ivar, passed on with `&@ivar` and
# instance_eval'd; an anonymous `Class.new(Args)` in `class << self` (decisions 145, 231).
module DemoCli
  class Args
    extend T::Sig

    def initialize
      @table = T.let({}, T::Hash[Symbol, T.untyped])
    end

    sig { params(name: Symbol, value: T.untyped).void }
    def set_arg(name, value)
      @table[name] = value
    end

    def kind = self.class.superclass.name
  end

  class AbstractCommand; end

  class Parser
    extend T::Sig

    sig { params(cmd: T.class_of(DemoCli::AbstractCommand), block: T.nilable(T.proc.bind(Parser).void)).void }
    def initialize(cmd, &block)
      @args = T.let((cmd.args_class || Args).new, Args)
      instance_eval(&block) if block
    end

    sig { params(name: Symbol).void }
    def switch(name)
      @args.set_arg(name, true)
    end

    sig { returns(Args) }
    def parse = @args
  end

  class AbstractCommand
    extend T::Sig

    class << self
      extend T::Sig

      sig { returns(T.nilable(T.class_of(Args))) }
      attr_reader :args_class

      sig { returns(Parser) }
      def parser = Parser.new(self, &@parser_block)

      private

      sig { params(block: T.proc.bind(Parser).void).void }
      def cmd_args(&block)
        @parser_block = T.let(block, T.nilable(T.proc.void))
        @args_class = T.let(const_set(:Args, Class.new(Args)), T.nilable(T.class_of(Args)))
      end
    end

    sig { returns(Args) }
    attr_reader :args

    def initialize
      @args = T.let(self.class.parser.parse, Args)
    end
  end

  class InfoCmd < AbstractCommand
    cmd_args do
      switch :json
    end

    def run = [args.kind, args.instance_variable_get(:@table)]
  end
end

# A T::Struct whose props a load-time loop declares under computed names: boot's props (decision 241).
class DemoFormulaStruct < T::Struct
  PREDICATES = [:bottle, :head].freeze

  PREDICATES.each do |predicate_name|
    present_method_name = :"#{predicate_name}_present"
    predicate_method_name = :"#{predicate_name}?"

    const present_method_name, T::Boolean, default: false

    define_method(predicate_method_name) do
      send(present_method_name)
    end
  end

  const :desc, String
end

# A subclass's class method widening a sig's T::Array[Entry] to T::Array[Object]: Object is untyped (decision 246).
module DemoBundle
  class Entry
    extend T::Sig
    sig { params(name: String).void }
    def initialize(name) = @name = name
    sig { returns(String) }
    attr_reader :name
  end

  class Ext
    class << self
      extend T::Sig
      sig { params(entries: T::Array[Entry]).returns(T::Array[String]) }
      def cleanup_items(entries) = entries.map(&:name)

      sig { params(entries: T::Array[Entry]).returns(T::Array[String]) }
      def run(entries) = cleanup_items(entries)
    end
  end

  class Vscode < Ext
    class << self
      extend T::Sig
      sig { params(entries: T::Array[Object]).returns(T::Array[String]) }
      def cleanup_items(entries) = entries.map { |e| T.cast(e, Entry).name.upcase }
    end
  end
end

# `@name = T.let(name, String)` of a T.nilable(String) param checked or reassigned on every path: unwrapped, nil
# raising TypeError as sorbet-runtime's T.let does (decision 248).
class DemoBottle
  extend T::Sig

  sig { params(formula: T.nilable(String), name: T.nilable(String)).void }
  def initialize(formula, name: nil)
    if formula
      name = formula.upcase
    else
      raise ArgumentError, "Bottle name is required" if name.nil?
    end
    @name = T.let(name, String)
  end

  sig { returns(String) }
  attr_reader :name

  sig { params(v: T.nilable(String)).returns(String) }
  def self.strict(v) = @v = T.let(v, String)
end

# Homebrew's SystemCommand#each_line_from: a T::Array[IO] param reassigned a Hash and iterated with each_key. The
# widened copy is found by a run the old type then fails, so that run is redone, and the union's Array arm raises
# NoMethodError if taken (decision 249).
class DemoLineSource
  extend T::Sig

  sig { params(sources: T::Array[String], _block: T.proc.params(type: Symbol, line: String).void).void }
  def each_line_from(sources, &_block)
    sources = {
      sources[0] => :stdout,
      sources[1] => :stderr,
    }
    until sources.empty?
      src = sources.keys.first
      yield(sources.fetch(src), src)
      sources.delete(src)
    end
    sources.each_key(&:upcase)
  end
end

# Homebrew's FormulaStruct.from_hash: `hash.transform_keys(&:to_sym).slice(*decorator.all_props)` (decision 249).
class DemoFormulaStruct
  extend T::Sig

  sig { params(formula_hash: T::Hash[String, T.untyped]).returns(DemoFormulaStruct) }
  def self.from_hash(formula_hash)
    formula_hash = formula_hash.transform_keys(&:to_sym).slice(*decorator.all_props)
    new(**formula_hash)
  end
end

# Homebrew's Version: Comparable with a `<=>` typed T.nilable(Integer) over Object's. Its Op_cmp slot is
# emitCmpAdapter's alone, not also a dynamic override slot (decision 249).
class DemoVersion
  extend T::Sig
  include Comparable

  sig { params(v: Integer).void }
  def initialize(v) = @v = v

  sig { returns(Integer) }
  attr_reader :v

  sig { params(other: T.anything).returns(T.nilable(Integer)) }
  def <=>(other)
    return unless other.is_a?(DemoVersion)

    v <=> other.v
  end
end

# Homebrew's SystemCommand: `@input = T.let(Array(input), T::Array[String])`, an Array[untyped] converted (decision 250).
class DemoSystemCommand
  extend T::Sig

  sig { params(input: T.any(String, T::Array[String])).void }
  def initialize(input) = @input = T.let(Array(input), T::Array[String])

  sig { returns(T::Array[String]) }
  attr_reader :input
end
