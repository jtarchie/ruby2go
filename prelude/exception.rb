# prelude/exception.rb
# rbs_inline: enabled
#
# The exception hierarchy.

class Exception < Object
  # @rbs @cause: Exception?
  # @rbs @__pcs: untyped
  # @rbs @__bt: Array[String]?
  # @rbs @__seen: bool

  #: (?String?) -> void
  def initialize(message = nil)
    @message = message
  end

  # The exception being handled when this one was raised: `raise` inside a
  # rescue clause sets it (lexically; a raise in a method the clause calls
  # does not, decision 103).
  #: () -> Exception?
  def cause = @cause

  #: (Exception) -> void
  def __set_cause(c)
    @cause = c unless @cause
  end

  # MRI order: message calls to_s, so overriding to_s changes message too.

  #: () -> String
  def to_s
    m = @message
    return m if m
    __class_name
  end

  #: () -> String
  def message = to_s

  #: () -> String
  def inspect
    s = to_s
    return __class_name if s.empty?
    return "#<#{__class_name}:#{s.inspect}>" if s.include?("\n")
    "#<#{__class_name}: #{s}>"
  end

  # The frames where this was raised, once a rescue bound it (decision 106); nil before, as MRI's is for an exception never raised.
  #: () -> Array[String]?
  def backtrace = %x{ return rbBacktrace(self) }

  #: (Array[String]) -> Array[String]
  def set_backtrace(bt)
    @__bt = bt
    bt
  end
end

class StandardError < Exception; end
class IOError < StandardError; end

class SystemCallError < StandardError; end

module Errno
  class EINVAL < SystemCallError; end
  class ENOENT < SystemCallError; end
  class ECHILD < SystemCallError; end
  class EEXIST < SystemCallError; end
  class EISDIR < SystemCallError; end
  class ENOTDIR < SystemCallError; end
  class EACCES < SystemCallError; end
  class ENOTEMPTY < SystemCallError; end
end

class RuntimeError < StandardError; end

class FrozenError < RuntimeError; end

class ArgumentError < StandardError; end

class TypeError < StandardError; end

class NameError < StandardError; end

class NoMethodError < NameError; end

class IndexError < StandardError; end

# key and receiver are the Hash#fetch (or ENV.fetch) that failed, as MRI's.
class KeyError < IndexError
  # @rbs @key: untyped
  # @rbs @receiver: untyped
  # @rbs @has_key: bool

  #: () -> untyped
  def key
    raise ArgumentError, "no key is available" unless @has_key

    @key
  end

  #: () -> untyped
  def receiver
    raise ArgumentError, "no receiver is available" unless @has_key

    @receiver
  end

  #: (String, untyped, untyped) -> KeyError
  def self.__for(message, receiver, key)
    e = new(message)
    e.__set(receiver, key)
    e
  end

  #: (untyped, untyped) -> void
  def __set(receiver, key)
    @receiver = receiver
    @key = key
    @has_key = true
  end
end

class StopIteration < IndexError; end

# Kernel#throw raises it when no active catch has the tag (decision 91).
class UncaughtThrowError < ArgumentError
  #: (String, untyped, untyped) -> void
  def initialize(message, tag, value)
    @message = message
    @tag = tag
    @value = value
  end

  #: () -> untyped
  def tag = @tag

  #: () -> untyped
  def value = @value
end

class RangeError < StandardError; end

class FloatDomainError < RangeError; end

class ZeroDivisionError < StandardError; end

class ScriptError < Exception; end

class NoMemoryError < Exception; end

# Never raised by rb2go (signals end the program, decision 60); rescue clauses may name them.
# A signal received: `signo`, and "SIGTERM" as the message. An untrapped
# Ctrl-C raises Interrupt in the main thread's blocking calls (decision 60).
class SignalException < Exception
  # @rbs @signo: Integer

  #: (untyped) -> void
  def initialize(sig)
    @signo = __signo(sig)
    super("SIG#{__signame(@signo)}")
  end

  #: (untyped) -> Integer
  def __signo(sig) = %x{ return Integer(rbSignalArg(sig)) }

  #: (Integer) -> String
  def __signame(n) = %x{ return String(rbSignalName(syscall.Signal(n))) }

  #: () -> Integer
  def signo = @signo

  #: () -> String
  def signm = message
end

class Interrupt < SignalException
  #: (?String?) -> void
  def initialize(message = nil)
    super("INT")
    @message = message || "Interrupt"
  end
end

class NotImplementedError < ScriptError; end

# Kernel#exit raises it, so ensure blocks and `rescue Exception` run first;
# rbTopRecover exits with its status and prints nothing.
class SystemExit < Exception
  #: (Integer) -> void
  def initialize(status)
    @message = "exit"
    @status = status
  end

  #: () -> Integer
  def status = @status

  #: () -> bool
  def success? = @status == 0
end
