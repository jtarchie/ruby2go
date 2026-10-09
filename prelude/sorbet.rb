# prelude/sorbet.rb - rbs_inline: enabled - sorbet-runtime's run-time surface (#95, decision 196). `sig {}` and
# `T.let`/`T.cast` are read by the compiler as types and never reach here; what is left is what runs.

module T
  # `extend T::Sig` and friends: the DSL they add is compile-time only.
  module Sig
  end

  module Helpers
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
