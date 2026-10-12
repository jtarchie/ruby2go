require "json"
# Gem code rb2go's typing would reject, compiled leniently as rack's multipart parser and request are (decision 175).
module Demo
  class Gemish
    Pair = Struct.new :key, :value

    class Part < Struct.new(:body, :name)
      def label = "#{name}=#{body}"
    end

    KINDS = %w[text html].freeze

    attr_reader :state

    def initialize
      @state = :ready
    end

    def kind?(headers) = KINDS.include?(headers["kind"])

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
    def tab_for(n) = n.empty? ? nil : (n == "x" ? GemTab.empty : GemAbstractTab.empty)

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
end
