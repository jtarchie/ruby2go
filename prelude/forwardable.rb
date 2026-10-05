# rbs_inline: enabled
#
# Forwardable: `extend Forwardable` then def_delegators/def_delegator/
# delegate in a class body. The compiler writes each delegated method out
# as a def with its target's signature (decision 99); the module itself
# has nothing to do at run time.
module Forwardable
end

# SingleForwardable: the same forms after `extend SingleForwardable` in a
# class or module body define class methods (decision 132).
module SingleForwardable
end
