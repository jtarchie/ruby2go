require "delegate"
require "json"
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

# Homebrew's PackagesIndex.write!: `data` (a typed Hash) rebound to `{ "version" => 2, **fingerprint, **data }`, which
# its type does not hold; the value is generated as itself and the local widens (decision 252).
module DemoPackagesIndex
  extend T::Sig

  sig { returns(T::Hash[String, Integer]) }
  def self.fingerprint = { "size" => 3, "mtime" => 9 }

  sig { returns(T.nilable(T::Hash[String, T::Hash[String, [Integer, Integer]]])) }
  def self.build = { "top" => { "a" => [1, 2] } }

  def self.write!
    return if (data = build).nil?

    data = {
      "version" => 2,
      **fingerprint,
      **data,
    }
    data.keys
  end
end

# Homebrew's Utils::Curl: `system_command(..., **command_options)` of a Hash holding mixed values, into typed keywords
# (decision 255).
module DemoCurl
  extend T::Sig

  sig { params(args: String, print_stdout: T.any(T::Boolean, Symbol), secrets: T::Array[String], debug: T.nilable(T::Boolean)).returns(String) }
  def self.system_command(*args, print_stdout: false, secrets: [], debug: nil)
    "#{args.join(" ")} #{print_stdout} #{secrets.size} #{debug.inspect}"
  end

  def self.curl(secrets: [], print_stdout: false, debug: nil)
    command_options = { secrets:, print_stdout:, debug: }.compact
    system_command("curl", "-s", **command_options)
  end
end

# Homebrew's SystemCommand: an IO attr_writer's ivar left nil is tested at run time (`if (inheritance =
# @sandbox_inheritance)`), and a local Hash merged with an Integer key widens (decision 257).
class DemoSandboxCmd
  extend T::Sig

  sig { params(sandbox_inheritance: IO).returns(IO) }
  attr_writer :sandbox_inheritance

  def initialize = @sandbox_inheritance = T.let(nil, T.nilable(IO))

  def options
    opts = { in: 0 }
    if (inheritance = @sandbox_inheritance)
      opts = opts.merge(198 => inheritance)
    end
    opts.keys
  end
end

# Homebrew's APIHashable#deep_remove_placeholders: `T.type_parameter(:U)` in, U out, through a case whose else arm is
# the value itself; U is a member of the case's union, boxed, not checked by a package-level helper (decision 258).
module DemoDeep
  extend T::Sig

  sig { type_parameters(:U).params(value: T.type_parameter(:U)).returns(T.type_parameter(:U)) }
  def self.deep(value)
    value = case value
    when Hash
      value.transform_values { |v| deep(v) }
    when Array
      value.map { |v| deep(v) }
    when String
      value.upcase
    else
      value
    end
    value
  end

  def self.go
    h = T.let({ "a" => "x" }, T::Hash[String, T.untyped])
    h["l"] = ["y", 1, { "b" => "z" }]
    [deep(h), deep(3), deep("q")]
  end
end

# Homebrew's `def bottle(&block) = stable.bottle(&block)`: a T.proc.bind block forwarded through an untyped receiver (decision 281)
class DemoBoundBottle
  extend T::Sig

  attr_reader :root

  sig { params(v: String).void }
  def root_url(v)
    @root = v
  end
end

class DemoBoundSpec
  extend T::Sig

  sig { returns(DemoBoundBottle) }
  def bs = @bs ||= DemoBoundBottle.new

  sig { params(block: T.proc.bind(DemoBoundBottle).void).void }
  def bottle(&block)
    bs.instance_eval(&block)
  end
end

class DemoBoundFormula
  class << self
    extend T::Sig

    sig { params(block: T.nilable(T.proc.bind(DemoBoundSpec).void)).returns(T.untyped) }
    def stable(&block)
      @stable ||= DemoBoundSpec.new
      return @stable unless block

      @stable.instance_eval(&block)
    end

    sig { params(block: T.proc.bind(DemoBoundBottle).void).void }
    def bottle(&block) = stable.bottle(&block)

    sig { params(block: T.nilable(T.proc.bind(DemoBoundSpec).void)).returns(T.untyped) }
    def via(&block) = T.unsafe(self).stable(&block)
  end
end

# Homebrew's Formula#initialize: `self.class.instance_method(:brew).owner` on a method whose block is required (decision 281)
class DemoNeedsBlockBase
  extend T::Sig

  sig { params(fetch: T::Boolean, _blk: T.proc.params(arg0: DemoNeedsBlockBase, arg1: String).void).void }
  def brew(fetch: true, &_blk)
    yield self, "x"
  end

  def ok? = self.class.instance_method(:brew).owner == DemoNeedsBlockBase
end

class DemoNeedsBlockSub < DemoNeedsBlockBase; end

# Homebrew's FormulaStruct: a tuple-typed T::Struct prop from parsed JSON, its Hash element converted (decision 281)
class DemoUrlStruct < T::Struct
  extend T::Sig

  const :url_args, [String, T::Hash[Symbol, T.anything]], default: ["", {}]
  const :desc, String

  sig { params(json: String).returns(DemoUrlStruct) }
  def self.deserialize(json)
    h = JSON.parse(json)
    if (args = h["url_args"])
      h["url_args"] = [args[0], args[1].transform_keys(&:to_sym)]
    end
    new(**h.transform_keys(&:to_sym).slice(*decorator.all_props).compact)
  end
end

# Homebrew's GitHubPackages.version_rebuild: an Integer param rebound to a String? (decision 281)
module DemoRebind
  extend T::Sig

  sig { params(version: String, rebuild: Integer, bottle_tag: T.nilable(String)).returns(String) }
  def self.version_rebuild(version, rebuild, bottle_tag = nil)
    bottle_tag = (".#{bottle_tag}" if bottle_tag)

    rebuild = if rebuild.positive?
      if bottle_tag
        ".#{rebuild}"
      else
        "-#{rebuild}"
      end
    end

    "#{version}#{bottle_tag}#{rebuild}"
  end
end

# Homebrew's Formulary.class_s: a String bang method with a block (decision 281)
module DemoClassS
  def self.class_s(name)
    class_name = name.capitalize
    class_name.gsub!(/[-_.\s][a-zA-Z0-9]/) { |matched| matched.chars.fetch(-1).upcase }
    class_name.tr!("+", "x")
    class_name.sub!(/(.)@(\d)/, "\\1AT\\2")
    class_name
  end
end

# Homebrew's DependencyCollector: @@x set in a prepended module's body, in gem code (decision 281)
class DemoCvCollector
  def initialize
    init!
  end

  def init!; end
end

module DemoCvLinux
  @@building = false

  def building! = @@building = true

  def building? = @@building

  def init!
    building?
  end
end

DemoCvCollector.prepend(DemoCvLinux)

# Homebrew's Dependencies: a SimpleDelegator typed by its initialize's super(args) (decision 284)
class DemoDlgDep
  extend T::Sig

  sig { returns(String) }
  attr_reader :name

  sig { params(name: String, build: T::Boolean).void }
  def initialize(name, build)
    @name = name
    @build = build
  end

  sig { returns(T::Boolean) }
  def build? = @build
end

class DemoDlgDeps < SimpleDelegator
  extend T::Sig

  sig { params(args: DemoDlgDep).void }
  def initialize(*args)
    super(args)
  end

  sig { returns(T::Array[DemoDlgDep]) }
  def build
    __getobj__.select(&:build?)
  end
end

module DemoDlg
  def self.go
    d = DemoDlgDeps.new(DemoDlgDep.new("a", true), DemoDlgDep.new("b", false))
    [d.select(&:build?).map(&:name), d.reject(&:build?).map(&:name), d.filter_map { |x| x.name if x.build? }, d.build.size, d.size]
  end
end

# Homebrew's Version < "2.39": Comparable's op with a <=> that takes untyped (decision 287)
class DemoCmpVersion
  extend T::Sig
  include Comparable

  sig { returns(String) }
  attr_reader :v

  sig { params(v: String).void }
  def initialize(v)
    @v = v
  end

  sig { params(other: T.untyped).returns(T.nilable(Integer)) }
  def <=>(other)
    other = DemoCmpVersion.new(other) if other.is_a?(String)
    return unless other.is_a?(DemoCmpVersion)

    v.split(".").map(&:to_i) <=> other.v.split(".").map(&:to_i)
  end
end

module DemoCmp
  def self.go
    a = DemoCmpVersion.new("2.35")
    err = begin
      a < 3
    rescue ArgumentError => e
      e.message
    end
    [a < "2.39", a >= "2.39", a > DemoCmpVersion.new("2.1"), err]
  end
end

# Homebrew's Libstdcxx.system_version: a bang method on a fresh String (`x.to_s` of a non-String) only answers its
# value, nil when nothing changed (decision 290)
module DemoFreshBang
  def self.go
    a = "6#{Pathname.new("/x/libstdc++.so.6.0.33").basename.to_s.delete_prefix!("libstdc++.so.6")}"
    b = "6#{Pathname.new("/x/other.so").basename.to_s.delete_prefix!("libstdc++.so.6")}"
    c = 42.to_s.sub!("4", "x")
    [a, b, c]
  end
end

# Homebrew's Resource::Patch#initialize: `super "patch", &block` where the subclass's block binds the subclass and the
# parent's binds the parent (decision 290)
class DemoRes
  extend T::Sig

  sig { returns(String) }
  attr_reader :name

  sig { params(name: String, block: T.nilable(T.proc.bind(DemoRes).void)).void }
  def initialize(name, &block)
    @name = name
    instance_eval(&block) if block
  end
end

class DemoResPatch < DemoRes
  sig { returns(T::Array[String]) }
  attr_reader :files

  sig { params(block: T.nilable(T.proc.bind(DemoResPatch).void)).void }
  def initialize(&block)
    @files = T.let([], T::Array[String])
    super("patch", &block)
  end

  sig { params(file: String).void }
  def apply(file)
    @files << file
  end
end

module DemoBoundBlk
  def self.go
    r = DemoResPatch.new { apply "a.diff" }
    [r.name, r.files]
  end
end

# Homebrew's Version::Token.create: `klass.new(val)` where klass is one of several subclasses whose new takes other
# types (decision 290)
class DemoTok
  extend T::Sig

  sig { returns(T.untyped) }
  attr_reader :value

  sig { params(value: T.nilable(T.any(String, Integer))).void }
  def initialize(value)
    @value = T.let(value, T.untyped)
  end

  sig { params(val: String).returns(DemoTok) }
  def self.create(val)
    case val
    when /\A\d+\z/ then DemoNumTok
    else DemoStrTok
    end.new(val)
  end
end

class DemoStrTok < DemoTok
  sig { params(value: String).void }
  def initialize(value)
    super(value.to_s)
  end
end

class DemoNumTok < DemoTok
  sig { params(value: T.any(String, Integer)).void }
  def initialize(value)
    super(value.to_i)
  end
end

module DemoTokNew
  def self.go = [DemoTok.create("12").value, DemoTok.create("rc").value, DemoTok.create("7").class.name]
end

# Homebrew's Downloadable#initialize_dup: a module's initialize_dup calling super with no class one above it reaches
# Object's (decision 290)
module DemoDupMix
  def initialize_dup(other)
    super
    @tags = @tags.dup
  end
end

class DemoDupped
  include DemoDupMix

  attr_reader :tags

  def initialize
    @tags = ["a"]
  end
end

module DemoDupSuper
  def self.go
    a = DemoDupped.new
    b = a.dup
    b.tags << "b"
    [a.tags, b.tags]
  end
end

# Homebrew's Downloadable `attr_reader :checksum` and Resource `attr_writer :checksum`: one @checksum (decision 291)
module DemoDl
  extend T::Sig

  sig { returns(T.nilable(String)) }
  attr_reader :checksum

  sig { void }
  def initialize
    @checksum = T.let(nil, T.nilable(String))
  end
end

class DemoDlRes
  extend T::Sig
  include DemoDl

  sig { params(checksum: T.nilable(String)).returns(T.nilable(String)) }
  attr_writer :checksum
end

module DemoSharedIvar
  def self.go
    r = DemoDlRes.new
    r.checksum = "abc"
    r.checksum
  end
end

# A local named `_` written in a branch and again after it is hoisted, and never read (decision 293)
module DemoHoistUnderscore
  def self.go(flag)
    if flag
      _, b = [1, 2]
    else
      b = 4
    end
    _, c = [5, 6]
    [b, c]
  end
end

# Homebrew's Requirements: a SimpleDelegator whose initialize wraps `Set.new(args)` (decision 291)
class DemoReqs < SimpleDelegator
  extend T::Sig

  sig { params(args: String).void }
  def initialize(*args)
    super(Set.new(args))
  end
end

module DemoSetDlg
  def self.go
    r = DemoReqs.new("a", "b", "a")
    [r.to_a, r.select { |x| x == "b" }.to_a, r.to_a.empty?, r.size]
  end
end

# Homebrew's extend/blank: TrueClass and FalseClass each answer present?; through an untyped value false must take
# FalseClass's, not Object's (decision 291)
class Object
  def demo_present? = true
end

class FalseClass
  def demo_present? = false
end

class TrueClass
  def demo_present? = true
end

module DemoBoolDyn
  def self.go
    vals = [false, true, "x"] #: Array[untyped]
    vals.map { |v| v.demo_present? }
  end
end
