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
