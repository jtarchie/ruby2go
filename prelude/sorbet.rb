# prelude/sorbet.rb - rbs_inline: enabled - sorbet-runtime's run-time surface (#95, decision 196). `sig {}` and
# `T.let`/`T.cast` are read by the compiler as types and never reach here; what is left is what runs.

module T
  # `extend T::Sig` and friends: the DSL they add is compile-time only.
  module Sig
  end

  module Helpers
  end

  # A T::Struct's typed attrs and keyword initialize are generated at compile time (decision 196).
  class Struct
  end

  class ImmutableStruct < Struct
  end

  # Its subclasses' `enums do X = new end` block is their body, each value named by the compiler (sorbetEnum).
  # ponytail: no values/deserialize/each_value; add them as a class-level list when a program reads one.
  class Enum
    # @rbs @serialized_val: untyped
    # @rbs @const_name: String?

    #: (?untyped) -> void
    def initialize(serialized_val = nil)
      @serialized_val = serialized_val
      @const_name = nil
    end

    # sorbet-runtime's _bind_name: the constant's name, and the serialized value it defaults to
    #: (String) -> self
    def __enum_bind(name)
      @const_name = name
      @serialized_val = name.downcase if @serialized_val.nil?
      self
    end

    #: -> untyped
    def serialize = @serialized_val

    #: -> String
    def to_s = inspect

    #: -> String
    def inspect = "#<#{self.class.name}::#{@const_name}>"
  end

  # sorbet-runtime's settings (Homebrew's standalone/sorbet.rb silences it): signatures are checked at compile time, so
  # its handlers never run.
  module Configuration
    #: (untyped) -> void
    def self.call_validation_error_handler=(handler); end

    #: (untyped) -> void
    def self.inline_type_error_handler=(handler); end

    #: () -> void
    def self.enable_final_checks_on_hooks; end
  end

  # Raises as sorbet-runtime does on nil.
  #: [X] (X?) -> X
  def self.must(arg)
    raise TypeError, "Passed `nil` into T.must" if arg.nil?

    arg
  end

  #: (untyped) -> untyped
  def self.unsafe(value) = value
end
