# prelude.rb
# rbs_inline: enabled
#
# The core library, compiled by the same transpiler as user code. It reopens
# core classes, so it never runs on MRI. Only `%x{}` leaves drop into Go.
# Each file is pulled in with require_relative; order matters only for the
# order of the emitted Go. This is the core: what MRI 4.0 defines without a
# require. Libraries (JSON, CSV, Date, ...) load only when the program
# requires them: internal/compiler/libs.go maps each require name to its
# prelude file, and each lib file require_relatives what MRI's own does
# (decision 155).

require_relative "prelude/runtime"
require_relative "prelude/object"
require_relative "prelude/module"
require_relative "prelude/method"
require_relative "prelude/struct"
require_relative "prelude/dynamic"
require_relative "prelude/boolean"
require_relative "prelude/numeric"
require_relative "prelude/integer"
require_relative "prelude/float"
require_relative "prelude/math"
require_relative "prelude/rational"
require_relative "prelude/random"
require_relative "prelude/complex"
require_relative "prelude/string"
require_relative "prelude/symbol"
require_relative "prelude/regexp"
require_relative "prelude/enumerable"
require_relative "prelude/array"
require_relative "prelude/enumerator"
require_relative "prelude/range"
require_relative "prelude/hash"
require_relative "prelude/set"
require_relative "prelude/time"
require_relative "prelude/exception"
require_relative "prelude/encoding"
require_relative "prelude/marshal"
require_relative "prelude/io"
require_relative "prelude/file"
require_relative "prelude/pathname"
require_relative "prelude/thread"
require_relative "prelude/fiber"
require_relative "prelude/queue"
require_relative "prelude/ractor"
require_relative "prelude/process"
require_relative "prelude/gc"
require_relative "prelude/monitor"
require_relative "prelude/pp"
