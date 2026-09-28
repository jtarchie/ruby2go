# prelude/singleton.rb
# rbs_inline: enabled
#
# `include Singleton` in a class gets it a memoized `instance` from the
# compiler (addSingletonInstance); the module itself is a marker.

module Singleton
end
