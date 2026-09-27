# prelude.rb
# rbs_inline: enabled
#
# The core library, compiled by the same transpiler as user code. It reopens
# core classes, so it never runs on MRI. Only `%x{}` leaves drop into Go.
# Each file is pulled in with require_relative; order matters only for the
# order of the emitted Go.

require_relative "prelude/runtime"
require_relative "prelude/object"
require_relative "prelude/module"
require_relative "prelude/struct"
require_relative "prelude/dynamic"
require_relative "prelude/boolean"
require_relative "prelude/integer"
require_relative "prelude/float"
require_relative "prelude/string"
require_relative "prelude/symbol"
require_relative "prelude/regexp"
require_relative "prelude/enumerable"
require_relative "prelude/array"
require_relative "prelude/range"
require_relative "prelude/hash"
require_relative "prelude/time"
require_relative "prelude/exception"
require_relative "prelude/json"
require_relative "prelude/thread"
require_relative "prelude/uri"
require_relative "prelude/net_http"
require_relative "prelude/webrick"
