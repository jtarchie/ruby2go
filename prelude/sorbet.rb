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
