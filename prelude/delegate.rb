# rbs_inline: enabled

# Delegation (decision 118). A method a Delegator subclass doesn't define is
# compiled to the same call on __getobj__: dynamic for SimpleDelegator,
# whose object is untyped, typed for DelegateClass(Foo), a class the
# compiler generates per Foo.
class Delegator < Object
end

class SimpleDelegator < Delegator
  #: (untyped) -> void
  def initialize(obj)
    @delegate_sd_obj = obj
  end

  #: () -> untyped
  def __getobj__ = @delegate_sd_obj

  #: (untyped) -> untyped
  def __setobj__(obj)
    @delegate_sd_obj = obj
  end

  # @dynamic
  #: () -> String
  def to_s = __getobj__.to_s

  # @dynamic
  #: () -> String
  def inspect = __getobj__.inspect

  # @dynamic
  #: (untyped) -> bool
  def ==(other) = other.equal?(self) || __getobj__ == other

  # @dynamic
  #: (untyped) -> bool
  def !=(other) = !(self == other)

  # @dynamic
  #: () -> Integer
  def hash = __getobj__.hash

  # @dynamic
  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = __getobj__.respond_to?(name)
end
