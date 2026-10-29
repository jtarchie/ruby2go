# rbs_inline: enabled

# Standalone: an instance delegation elsewhere would mask a stale class-object method cache (decision 132).
require "forwardable"

LIMIT = 5

module SingleOnly
  extend SingleForwardable
  def_delegator :names, :size
  def_delegator :names, :first
  LIMIT = "abc"
  def_delegator :LIMIT, :succ

  #: () -> Array[String]
  def self.names = ["a", "b"]
end

p SingleOnly.size, SingleOnly.first, SingleOnly.succ
