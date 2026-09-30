# prelude/exception.rb
# rbs_inline: enabled
#
# The exception hierarchy.

class Exception < Object
  #: (?String?) -> void
  def initialize(message = nil)
    @message = message
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

  #: () -> String?
  def backtrace = nil
end

class StandardError < Exception; end
class IOError < StandardError; end

class SystemCallError < StandardError; end

module Errno
  class EINVAL < SystemCallError; end
  class ENOENT < SystemCallError; end
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

class KeyError < IndexError; end

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
class SignalException < Exception; end

class Interrupt < SignalException; end

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
