require "pathname"
require "monitor"
require "json"
require "uri"
require "optparse"
# Gem code rugaroo's typing would reject, compiled leniently as rack's multipart parser and request are (decision 175).
module Demo
  class Gemish
    Pair = Struct.new :key, :value

    Part = Struct.new(:body, :name) do
      def label = "#{name}=#{body}"
    end

    KINDS = %w[text html].freeze

    # bindata's self-replacing `inherited` hook (decision 301)
    module Registry
      @names = []

      def self.register(name) = @names << name
      def self.names = @names.sort
    end

    class Base
      class << self
        def register_subclasses
          singleton_class.send(:undef_method, :inherited)
          define_singleton_method(:inherited) do |subclass|
            Registry.register(subclass.name)
            register_subclasses
          end
        end

        private :register_subclasses
      end

      register_subclasses
    end

    # ruby-macho's `private def inherited` on the class-level (decision 301): the runtime calls a private hook
    class Guarded
      @seen = []
      @fields = []

      class << self
        attr_reader :seen, :fields

        def field(name) = @fields << name

        private

        def inherited(subclass)
          super
          Guarded.seen << subclass.name
          fields = @fields.dup
          subclass.class_eval do
            @fields = fields
          end
        end
      end
    end

    class Guard1 < Guarded
      field :a
    end

    class Guard2 < Guard1
      field :b
    end

    class Leaf < Base; end
    class Twig < Leaf; end

    attr_reader :state

    def initialize
      @state = :ready
    end

    def kind?(headers) = KINDS.include?(headers["kind"])

    # bindata's lazy evaluator: an alias of the respond_to? intrinsic (decision 301)
    def safe_responds?(sym) = base_respond_to?(sym, false)
    alias base_respond_to? respond_to?

    # A local rebound to a Hash built by a block that declares its own locals (FormulaInstaller#forbidden_license_check, decision 301)
    def rebind(text)
      words = text.to_s.dup
      words = words.split.each_with_object({}) do |word, hash|
        sym = word.to_sym
        word = sym if sym == :a
        hash[word] = 1
      end
      words
    end

    # A block parameter rebound to the call's result (concurrent-ruby's TimerSet#post)
    def rebind_block(count, &task)
      task = Pair.new("k", count + task.call)
      task.value > 1 ? task : false
    end

    # a call a class never gets (Cask::Cask#tab when cask/cask.rb was not loaded): found, or not, at run time (decision 301)
    def never_defined(pair) = Pair.new("k", 1).no_such_thing(pair)

    # a Hash literal widened by a later write of another value type (SystemCommand#each_output_line, decision 323)
    def spawn_opts(dir)
      options = { pgroup: true }
      options[:chdir] = dir if dir
      options
    end

    # a module's zero-argument override of a Pathname method taking an optional keyword (ObserverPathnameExtension#mkpath)
    module ObservedMkpath
      def mkpath
        super
        Gemish.mkpaths << to_s
      end
    end

    class ObservedPath < Pathname
      include ObservedMkpath
    end

    def self.mkpaths = (@mkpaths ||= [])

    def observed_mkpath(dir)
      ObservedPath.new(File.join(dir, "a", "b")).mkpath
      [File.directory?(File.join(dir, "a", "b")), Gemish.mkpaths.size]
    end

    # `Gemish.extend Factory` inside the body (bindata's `BinData.extend BitFieldFactory`)
    module Factory
      def factory_name = "factory #{name}"
    end
    Gemish.extend Factory

    # a splat of a value that is a Symbol or an Array (bindata's BitField.define_class(name, :nbits, *args))
    def splat_args
      made = []
      { "Bit" => :big, "Sbit" => %i[big signed] }.each_pair { |name, args| made << define_field(name, :nbits, *args) }
      made
    end

    def define_field(name, nbits, endian, signed = :unsigned) = [name, nbits, endian, signed]

    def joined
      out = String.new
      out << "a" << "b" << "c"
      out
    end

    def first_big(xs)
      xs.each { |x| return x if x > 2 }
      nil
    end

    def recheck
      Integer("x")
    rescue ArgumentError => e
      raise e.class, e.message, cause: nil
    end

    # Blocks on JSON's untyped values: a Hash takes Hash's method, an Array Array's (decision 224).
    def json_kids(json) = JSON.parse(json)["kids"].select { |k| k["age"] > 3 }.map { |k| k["name"] }

    def json_find(json) = JSON.parse(json)["sigs"]&.find { |s| s.dig("h", "kid") == "b" }

    def json_big(json) = JSON.parse(json).select { |_, v| v > 1 }

    # A sig narrower than the value (a parent class, nil, another union): Ruby checks none of it (decision 225).
    #: (String) -> Demo::GemTab
    def tab_for(n) = if n.empty?
                       nil
                     else
                       (n == "x" ? GemTab.empty : GemAbstractTab.empty)
                     end

    # A yield inside a block, its value the method's (decision 226).
    def synced = synchronize { @state }

    def synchronize
      @lock ||= Thread::Mutex.new
      @lock.owned? ? yield : @lock.synchronize { yield }
    end

    # A tuple's Array methods: `[t - 1, 0.0].max` (decision 227).
    def remaining(t) = [t - 1, 0.0].max

    # Read before anything assigns it: untyped, nil (decision 220).
    def never_set_or(d) = @never_set || d

    def self.cached = @cache

    # A computed send in gem code reaches what gem and user code define, inherited and Kernel names (decision 235).
    def dyn_send(obj, name, *args) = obj.send(name, *args)

    def dyn_responds(obj, name) = obj.respond_to?(name)

    # An optional dependency that is not loaded: MRI raises NameError only if the read runs (decision 218).
    def setup_optional(on)
      return :skipped unless on

      ::NotLoadedOptionalDep.setup(x: 1)
    end
  end

  class GemAbstractTab
    def self.empty = new

    def kind = "abstract"
  end

  class GemTab < GemAbstractTab
    def kind = "tab"
  end

  # `$stdout.extend(M)`: IO answers M's methods (decision 229).
  module Out
    module Mixin
      def ohai(msg) = puts("==> #{msg}")
    end

    $stdout.extend Mixin
  end

  # A per-flag `define_singleton_method(name)` template: CliArgs answers any name through it (decision 230).
  class CliArgs
    def initialize
      @table = {} #: Hash[Symbol, untyped]
    end

    def set_arg(name, value)
      @table[name] = value
    end
  end

  class CliParser
    def initialize
      @args = CliArgs.new #: CliArgs
    end

    def parse(flags)
      flags.each { |f| set_args_method(:"#{f}?", true) }
      set_args_method(:sizes?, false)
      @args
    end

    def set_args_method(name, value)
      @args.set_arg(name, value)
      return if @args.respond_to?(name)

      @args.define_singleton_method(name) do
        instance_variable_get(:@table).fetch(name)
      end
    end
  end

  # `ENV.extend(M)`: ENV answers M's methods; M's select on self is a Hash's (decisions 229, 224).
  module EnvSensitive
    def sensitive?(key) = key.match?(/TOKEN|PASSWORD/)

    def sensitive_environment = select { |key, _| sensitive?(key) }
  end
  ENV.extend(EnvSensitive)

  # Decision 233's gem leniencies: an abstract parent taking its overrides' block, tuple arity, raise forms, a
  # local of two Array types, nil into a predicate, ivar String mutation and out-buffers.
  class AbstractLockable
    def synchronize
      raise NotImplementedError
    end
  end

  class MutexLockable < AbstractLockable
    def initialize = @lock = Thread::Mutex.new

    def synchronize
      if @lock.owned?
        yield
      else
        @lock.synchronize { yield }
      end
    end
  end

  class Counter < MutexLockable
    def initialize
      super
      @n = 0
    end

    def inc = synchronize { @n += 1 }

    def value = synchronize { @n }
  end

  class Lenient
    def self.quad(n) = [n, n.to_s, n * 2, :x]

    def self.three(n)
      a, b, c = quad(n)
      [a, b, c]
    end

    def self.pairs(n) = [quad(n)].map { |a, b| "#{a}#{b}" }

    def self.mixed(flag)
      xs = [1]
      xs = ["a"] if flag
      xs.size
    end

    def self.raised(k, msg)
      raise(k, msg)
    rescue StandardError => e
      [e.class.name, e.message]
    end

    def self.lines(text)
      r, w = IO.pipe
      w.write(text)
      w.close
      buf = +""
      out = []
      loop do
        r.read_nonblock(4, buf)
        out << buf.dup
      end
    rescue EOFError
      out.join
    end

    SEV_DEBUG = 0
    SEV_INFO = 1

    # A param typed from its default (:SEV_INFO), reassigned const_get's untyped value (decision 241).
    def self.level_of(level = :SEV_INFO)
      level = const_get(level) unless level.is_a?(Integer)
      level + 1
    end

    # Non-local return from closure blocks (Homebrew's Utils.popen: `IO.popen(...) do |pipe| return pipe.read ...`):
    # rescue Exception in the block passes it on, ensure runs, and a recursive call keeps its own (decision 247).
    def self.first_over(xs, limit, log)
      xs.map do |x|
        return x if x > limit
      rescue Exception # rubocop:disable Lint/RescueException
        log << "rescued"
        raise
      ensure
        log << "ensure #{x}"
      end
      nil
    end

    #: (Integer, Array[String]) -> String
    def self.depth_find(n, log)
      [n].map do |k|
        inner = n.positive? ? depth_find(n - 1, log) : :bottom
        log << "#{k}:#{inner}"
        return "at #{k}" if k.even?
      end
      "none #{n}"
    end

    def self.void_early(log)
      [1, 2, 3].each_with_object([]) do |x, acc|
        acc << x
        return if x == 2
      end
      log << "not reached"
    end

    # Homebrew's `missing_libraries, deps = formula.missing_library_linkage if installed; missing_libraries ||= []`
    # (decision 248).
    #: () -> [Array[String], Array[String]]
    def self.linkage = [["/a/libx.so"], ["dep"]]

    def self.missing(installed)
      libs, deps = linkage if installed
      libs ||= []
      deps ||= []
      libs.map { |lib| File.basename(lib) } + deps
    end

    # Homebrew's SoftwareSpec#uses_from_macos: `dep, tags = bounds.shift; tags = [*tags]` of an untyped value
    # (decision 249).
    def self.tags_of(bounds)
      _dep, tags = bounds.shift
      [*tags]
    end

    # A computed send of a prelude method no code calls by name, named as a symbol (decision 246).
    def self.sent(text, how: :swapcase) = text.public_send(how)

    # concurrent-ruby's logger: a lambda with a default and a `&block` param, its block nil (decision 241).
    def self.logger(level = 2)
      lambda do |severity, progname, message = nil, &block|
        return false if severity < level

        message = block ? block.call : message
        "#{progname}: #{message.inspect}"
      end
    end

    # `{build_env: true}.merge(options)` with options untyped: the result holds both (decision 241).
    def self.merged(options) = { build_env: true }.merge(options)

    # A local retyped String -> URI::Generic, and @x ||= on a union ivar (decision 241).
    def self.uri_path(ref)
      uri = ref.to_s
      return unless uri.start_with?("http")

      uri = URI(uri)
      uri.path
    end
  end
end

# concurrent-ruby's Dereferenceable: a mixin calling `synchronize { }` on self, which its includer defines (decision 243).
module DemoDeref
  def set_opts(opts = {})
    synchronize { ns_set(opts) }
  end

  def ns_set(opts)
    @dupv = opts[:dup]
    nil
  end

  def value_with(x)
    synchronize { |a, b| "#{x}-#{a.inspect}-#{b.inspect}" }
  end
end

class DemoLockable
  def initialize
    @m = Mutex.new
  end

  def synchronize
    @m.synchronize { yield 1, 2 }
  end
end

class DemoDelayish < DemoLockable
  include DemoDeref

  def initialize
    super
    set_opts(dup: true)
  end

  attr_reader :dupv
end

# optparse's shapes in gem code (decision 244): a computed send passing a block on, an untyped Method as a Proc, and
# `q, a = nil`.
class DemoVisitor
  def initialize = @stack = [DemoLockable.new]

  def visit(id, *args, &block)
    @stack.reverse_each { |el| el.__send__(id, *args, &block) }
    nil
  end

  def collect_yields
    got = []
    visit(:synchronize) { |a, b| got << [a, b] }
    got
  end

  def as_proc(x) = x.method(:upcase).to_proc

  def pair
    q, a = nil
    [q, a]
  end
end

# brew's `parser.on_tail(...) { }` on a parser held untyped: the Dyn wrapper's block is rewritten to __on_tail_flag's
# (bool) shape and stays a Proc (decision 307).
class DemoTailCli
  def initialize = @parts = { parser: OptionParser.new, n: 1 }

  def run(argv)
    seen = []
    @parts[:parser].on_tail("-h", "--help", "Show help") { |v| seen << v }
    @parts[:parser].on("-q") { |v| seen << [:q, v] }
    [@parts[:parser].parse(argv), seen]
  end
end

# concurrent-ruby's AbstractExecutorService: `ns_initialize(opts, &block)` inside synchronize, ns_initialize defined
# only by subclasses, which keep the block as a value (decision 248).
class DemoAbstractExec
  attr_reader :name

  def initialize(opts = {}, &block)
    @mon = Monitor.new
    @mon.synchronize do
      @name = opts.fetch(:name) if opts.key?(:name)
      ns_initialize(opts, &block)
    end
  end
end

class DemoPool < DemoAbstractExec
  def ns_initialize(opts, &block)
    @size = opts.fetch(:size, 2)
    @hook = block
  end

  def report = [@name, @size, @hook&.call(3)]
end

# concurrent-ruby's promises and executors: a block typed from its callers (`{ |x| }`) forwarded with `&task` where
# the callee's is (*untyped) -> untyped, and `task.call(*args)` spreading into it (decision 248).
class DemoBlocked
  def initialize(args, &task)
    raise ArgumentError, "no block given" unless block_given?

    @task = task
    @args = args
  end

  def run = @task.call(*@args)
end

class DemoThen < DemoBlocked
end

class DemoExec
  def post(*args, &task)
    raise ArgumentError, "no block given" unless block_given?

    ns_execute(*args, &task)
  end

  def ns_execute(*args, &task) = task.call(*args)
end

# Homebrew's EnvSensitive: a top-level `ENV.extend(M)` (ENV's class includes M), whose methods iterate self with
# `each_key do ... next ... end` and run a block with the environment changed (decision 248).
module DemoEnvSensitive
  def demo_sensitive?(key) = key.match?(/(token|password)/i)

  #: () ?{ () -> untyped } -> untyped
  def demo_clear_sensitive!(&block)
    unless block
      each_key do |key|
        next unless demo_sensitive?(key)

        self[key] = "{{DEFERRED:#{key}}}"
      end
      return
    end

    old_env = to_hash.dup
    begin
      demo_clear_sensitive!
      yield
    ensure
      replace(old_env)
    end
  end
end

ENV.extend(DemoEnvSensitive)

# concurrent-ruby's TimerSet#post(delay, *args, &task) over RubyExecutorService#post(*args, &task): no typed adapter
# fits, so the parent's slot boxes its arguments into a dynamic call of the override, as MRI calls it (decision 249).
class DemoPostExec
  def post(*args, &task)
    raise ArgumentError, "no block given" unless block_given?

    task.call(*args)
  end

  def run_all(xs) = xs.map { |x| post(x, 10) { |v| v + 1 } }
end

class DemoTimerSet < DemoPostExec
  def post(delay, *args, &task)
    raise ArgumentError, "no block given" unless block_given?

    [delay, task.call(*args)]
  end
end

# concurrent-ruby's ConditionSignalling#ns_broadcast (self) over AbstractLockableObject's abstract one: a module's
# override with its own Go name, the includer adapting the superclass's slot (decision 249).
class DemoAbstractLockable
  def ns_broadcast
    raise NotImplementedError
  end

  def wait_until
    yield
    ns_broadcast
  end
end

module DemoConditionSignalling
  def ns_broadcast
    @count = (@count || 0) + 1
    self
  end
end

class DemoMutexLockable < DemoAbstractLockable
  include DemoConditionSignalling
  attr_reader :count
end

class DemoEvent < DemoMutexLockable
  def set = wait_until { true }
end

# concurrent-ruby's RubyThreadPoolExecutor#ns_execute: a value block forwarded with `&task` to fallback_action, whose
# block is void and which runs it with `yield(*args)` (decision 250).
class DemoFallbackExec
  def initialize = @log = []
  attr_reader :log

  def fallback_action(*args)
    yield(*args)
    @log << :fallback
    nil
  end

  def ns_execute(*args, &task)
    fallback_action(*args, &task)
  end

  def go = ns_execute(5) do |x|
  @log << x
  x * 2
  end
end

# concurrent-ruby's IVar#add_observer(&block) over Concern::Observable's: the override takes the module slot's untyped
# block, and the stored block answers `public_send(:call, v)` (decision 250).
module DemoObservable
  def add_observer(observer = nil, func = :update, &block)
    demo_observers << [observer || block, func]
    observer || block
  end

  def demo_observers = @demo_observers ||= []

  def with_observer(observer = nil, func = :update, &block)
    add_observer(observer, func, &block)
    self
  end

  def demo_notify(v) = demo_observers.map { |o, f| o.public_send(f, v) }
end

class DemoIVar
  include DemoObservable

  def add_observer(observer = nil, func = :update, &block)
    raise ArgumentError, "both" if observer && block

    if block
      observer = block
      func = :call
    end
    super(observer, func)
  end
end

# concurrent-ruby's RubyThreadPoolExecutor#ns_initialize(opts): no block, though AbstractExecutorService#initialize
# passes one dynamically (`ns_initialize(opts, &block)`); MRI ignores it (decision 251).
class DemoPlainPool < DemoAbstractExec
  def ns_initialize(opts)
    @size = opts.fetch(:size, 1)
  end

  def size = @size
end

# concurrent-ruby's ExecutorService#post, included by AbstractExecutorService: a subclass's post(delay, *args, &task)
# answers the superclass's slot dynamically, though a module owns it (decision 251).
module DemoExecutorService
  def post(*_args, &_task)
    raise NotImplementedError
  end
end

class DemoAbstractService
  include DemoExecutorService

  def schedule(x) = post(x, 10) { |v| v + 1 }
end

class DemoTimerService < DemoAbstractService
  def post(delay, *args, &task)
    [delay, task.call(*args)]
  end
end

# concurrent-ruby's CachedThreadPool#ns_initialize: an `if Concurrent.on_jruby?` branch calling java.util..., which
# does not compile, raises if reached instead of costing the method; the override typed by its parent ends in nil
# there (decision 251).
class DemoPoolBase
  def ns_initialize(opts) = @size = opts.fetch(:size, 1)
end

class DemoCachedPool < DemoPoolBase
  def ns_initialize(opts)
    super(opts)
    @executor = java.util.concurrent.Executors.newCachedThreadPool if opts[:jruby]
  end

  def size = @size
end

# Homebrew's extend/blank.rb presence on a String?: nil's arm calls Object#presence on a typed nil NilClass, whose self
# result is nil, not a NilClass object Go would see as non-nil (`curlrc&.start_with?`, decision 252).
class Object
  def demo_blank? = respond_to?(:empty?) ? !!empty? : false
  def demo_presence = (self unless demo_blank?)
end

class NilClass
  def demo_blank? = true
end

module DemoEnvConfig
  module_function

  #: (String) -> String?
  def env_value(env) = ENV.fetch(env, nil)

  def curlrc = env_value("DEMO_CURLRC_UNSET").demo_presence
end

# Homebrew's extend/pathname.rb: `File.basename(self)` in a reopened Pathname, a value type answering to_path, where a
# path String is taken (decision 253).
class Pathname
  def demo_extname
    base = File.basename(self)
    return "" if base.match?(/\b\d+\.\d+[^.]*\Z/)

    File.extname(base)
  end

  def demo_stem = File.basename(self, demo_extname)
end

# concurrent-ruby pieces (decision 253): LockFreeStack::Node's `singleton_class.send :alias_method, :[], :new`;
# BlockedPromise.new_blocked_by1's `new(...)` on a subclass with its own initialize; a rest parameter given an
# untyped value and an Array; Event's `then` over Kernel#then; nil reaching an object-typed parameter and `nil?`.
class DemoNode
  attr_reader :value, :nxt

  def initialize(value, nxt)
    @value = value
    @nxt = nxt
  end

  singleton_class.send :alias_method, :[], :new
end

class DemoBlockedBase
  attr_reader :delayed

  def initialize(delayed, count)
    @delayed = delayed
    @count = count
  end

  def self.blocked_by(delayed, *args, &block) = new(delayed, 1, *args, &block)

  def chained? = @delayed.nil?
end

class DemoChainPromise < DemoBlockedBase
  def initialize(delayed, count, executor, args, &task)
    super(delayed, count)
    @executor = executor
    @args = args
    @task = task
  end

  def run = @task.call(*@args, @executor)
end

class DemoEventFuture
  def chain(*args) = [:chain, *args]
end

class DemoPromEvent < DemoEventFuture
  alias then chain
end

module DemoPromises
  def self.make(event) = event.then(1, 2)
  def self.chain(executor) = DemoChainPromise.blocked_by(nil, executor, [5]) { |a, e| [a, e] }
end

# concurrent-ruby's AbstractPromise#evaluate_to(*args, block), called on an untyped receiver: post parameters bound
# from the end of a dynamic call's arguments (decision 254).
class DemoEvaluate
  def evaluate_to(*args, block) = block.call(*args)
  def lead_post(a, *mid, z) = [a, mid, z]
end

module DemoEvaluateRun
  def self.go(p) = [p.evaluate_to(1, 2, ->(x, y) { x + y }), p.lead_post(1, 2, 3, 4), p.lead_post(1, 9)]
end

# concurrent-ruby's BlockedPromise.new_blocked_by1 → ChainPromise#initialize(…, &task): the dynamic new of a subclass
# passes initialize the block, and `super(…, *args, &task)` spreads an untyped args (decision 255).
class DemoBlocked2
  def initialize(delayed, _count, *args, &task)
    @delayed = delayed
    @args = args
    @task = task
  end

  def self.blocked_by(delayed, *args, &block) = new(delayed, 1, *args, &block)

  def run = @task ? @task.call(*@args) : :no_task
end

class DemoChain2 < DemoBlocked2
  def initialize(delayed, count, executor, args, &task)
    super(delayed, count, *args, &task)
    @executor = executor
  end
end

# Homebrew's SystemCommand#exec3: `options = options.merge(Sandbox::INHERITANCE_FD => fd)` of a **options param, an
# Integer key into Symbol keys: both sides untyped-keyed, the param widened (decision 256).
module DemoExec3
  FD = 3
  def self.exec3(inheritance, **options)
    options[:in] = 0
    options = options.merge(FD => inheritance) if inheritance
    options
  end
end

# concurrent-ruby's ThenPromise#initialize(…, &task) inherits BlockedTaskPromise's signature, its (*untyped) block
# whole: a block of four parameters gets all four values (decision 256).
class DemoTaskBase
  def initialize(args, &task)
    @args = args
    @task = task
  end

  def run(v) = @task.call(v, *@args)
end

class DemoThenTask < DemoTaskBase
end

# concurrent-ruby's ExecutorService#post(*args, &task): RubyExecutorService's override keeps `&task` as a value, so
# it is (*untyped) -> untyped whatever ImmediateExecutor's `yield(*args)` shape is; a two-parameter block gets both
# values (decision 257).
module DemoExecutorService
  def post(*args, &task)
    raise NotImplementedError
  end
end

class DemoImmediateExec
  include DemoExecutorService

  def post(*args)
    yield(*args)
    true
  end
end

class DemoRubyExec
  include DemoExecutorService

  def post(*args, &task)
    task.call(*args)
  end
end

module DemoPoster
  def self.go(ex) = ex.post([1, 2], :task) { |args, task| [args, task] }
end

# Homebrew's CaskDependent#deps: `depends_on.formula.map { |f| Dependency.new f }` on an untyped receiver; its Hash
# arm's pair where a String is wanted is checked at run time, not a Go type error (decision 258).
class DemoCaskDep
  #: (String) -> void
  def initialize(name) = @name = name
  def to_s = "dep:#{@name}"
end

class DemoCaskDeps
  def formula = @formula ||= ["wget"]
end

module DemoCaskDepsUser
  def self.deps(on) = on.formula.map { |f| DemoCaskDep.new f }.map(&:to_s)
end

# Homebrew's `class Pathname; alias to_str to_s; end`: a Pathname where a String is taken converts through that to_str
# (Mktemp#run's `Dir.chdir(@tmpdir) { ... }`, decision 258).
class Pathname
  alias to_str to_s
end

class DemoChdir
  #: (Pathname) -> void
  def initialize(dir) = @dir = dir
  def run = Dir.chdir(@dir) { Dir.pwd }
end

# Homebrew's Formulary.load_formula_from_struct!: `Class.new(::Formula) do ... end` in a method, its block running
# class-level DSL calls and ivar writes on the enclosing method's locals; each evaluation is a fresh class object with
# its own state, define_method a compile-time def, and inherited told of each (decision 259).
class DemoAPIFormula
  class << self
    attr_reader :loaded_from_api

    def inherited(child)
      super
      child.instance_eval do
        @loaded_from_api = false
        @desc = nil
      end
    end

    def desc(val = nil)
      return @desc if val.nil?

      @desc = val
    end

    def homepage(val = nil)
      return @homepage if val.nil?

      @homepage = val
    end
  end

  attr_reader :name

  def initialize(name)
    @name = name
  end

  def desc = self.class.desc
  #: () -> String?
  def caveats = nil
  def show = [name, desc, self.class.homepage, caveats, self.class.loaded_from_api]
end

class DemoFStruct
  attr_reader :desc, :homepage, :caveats

  def initialize(desc, homepage, caveats)
    @desc = desc
    @homepage = homepage
    @caveats = caveats
  end
end

module DemoFormulary
  def self.load(name, st)
    namespace = :"FormulaNamespaceAPI#{name.length}"
    mod = Module.new
    remove_const(namespace) if const_defined?(namespace)
    const_set(namespace, mod)
    mod.const_set(:BUILD_FLAGS, [])
    klass = Class.new(DemoAPIFormula) do
      @loaded_from_api = true
      desc st.desc
      homepage st.homepage
      @caveats_string = st.caveats
      define_method(:caveats) do
        self.class.instance_variable_get(:@caveats_string)
      end
    end
    mod.const_set(name.capitalize, klass)
    klass
  end
end

# no initialize of its own: Class.new's object still makes instances that know it
class DemoBareFormula
  class << self
    attr_accessor :tag
  end

  def tag = self.class.tag
end

module DemoBareLoader
  def self.load(tag)
    Class.new(DemoBareFormula) do
      self.tag = tag
    end
  end
end

module DemoAPIRun
  def self.run_api
    a = DemoFormulary.load("wget", DemoFStruct.new("get files", "https://wget", "none"))
    b = DemoFormulary.load("curl", DemoFStruct.new("transfer", "https://curl", nil))
    [a.new("wget").show, b.new("curl").show, a.desc, b.desc, DemoBareLoader.load("x").new.tag, DemoBareLoader.load("y").new.tag]
  end
end

# A typed Hash or Array probed with an untyped key of another class misses, as in MRI, rather than raising (decision 307).
class DemoKeyed
  def initialize
    @by = {}
    @list = []
  end

  def add(name)
    @by[name] = name.length
    @list << name
    self
  end

  def probe(k) = [@by[k], @by.key?(k), @by.include?(k), @by.fetch(k, -1), @list.include?(k), @list.index(k), @by.delete(k)]

  def probe_both = [probe(1), probe(:ab), probe("ab")]

  # fetch(k)'s miss is KeyError, fetch(k) { |x| }'s the block's value for x (decision 314)
  def probe_fetch(k)
    hit = begin
      @by.fetch(k).to_s
    rescue KeyError => e
      "#{e.class}: #{e.message} #{e.key.inspect} #{e.receiver.equal?(@by)}"
    end
    [hit, @by.fetch(k) { |x| -x.inspect.length }]
  end

  def probe_fetches = [probe_fetch(1), probe_fetch(:ab), probe_fetch("cde")]
end

# Homebrew's VerificationCache#verify: a leaf class's C? (decision 360) where C is wanted, nil checked only by a
# method rugaroo does not narrow by (`blank?`): the pointer goes on, nil as MRI's nil.
class DemoSum
  #: (String) -> void
  def initialize(hex) = @hex = hex
  def hex = @hex
end

class DemoSumError
  #: (DemoSum, DemoSum) -> void
  def initialize(want, got) = @msg = "#{want.hex} != #{got.hex}"
  def msg = @msg
end

module DemoVerify
  #: (DemoSum?) -> String
  def self.go(want)
    raise ArgumentError if want.to_s.empty?

    DemoSumError.new(want, DemoSum.new("bb")).msg
  end
end

# Homebrew's Livecheck::Strategy::Git: a class-level ivar its body sets, and a singleton attr_writer for it: one
# non-nil Hash; its writer still takes nil, as its sig says (decision 374 on 323).
class DemoUrlCache
  @seen = {} #: Hash[String, String]

  class << self
    attr_writer :seen #: Hash[String, String]

    def seen_count = @seen.size

    def seen_nil? = @seen.nil?
  end
end

# Homebrew's Cask::DSL#set_unique_stanza: a generic U bound to a C? box (a String subclass's, as DSL::Version's) by the
# caller's return type, the value back from instance_variable_set untyped: it converts to the box.
class DemoVer < String
  def kind = "ver #{self}"
end

class DemoStash
  def initialize
    @shape = nil #: DemoVer?
  end

  #: [U] (Symbol) { () -> untyped } -> U
  def put(name) = instance_variable_set(:"@#{name}", yield)

  #: () -> DemoVer?
  def shape = put(:shape) { DemoVer.new("1.2") }
end

# Homebrew's Cask::Installer#cask_and_formula_dependencies: an Array[untyped] (a tsort over two includers) into an
# Array[A | B]? ivar converts where it lands.
class DemoDepList
  def initialize
    @deps = nil #: Array[Integer | String]?
  end

  #: () -> Array[untyped]
  def raw = [1, "a", 2]

  def deps
    return @deps if @deps

    @deps = raw - [2]
  end
end
